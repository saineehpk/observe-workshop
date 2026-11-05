package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Outcome is the closed set every business record uses.
const (
	Accepted = "accepted"
	Rejected = "rejected"
	Failed   = "failed"
	Partial  = "partial"
)

// MoneyEffect answers "did money move?". unknown means we cannot tell
// (for example the caller timed out while the worker may still debit).
const (
	MoneyNone      = "none"
	MoneyConfirmed = "confirmed"
	MoneyUnknown   = "unknown"
)

// knownErrorClasses bounds the metric label. Anything else becomes "other"
// so a typo or a raw error string can never explode metric cardinality.
var knownErrorClasses = map[string]bool{
	"none": true, "unknown_user": true, "invalid_amount": true,
	"insufficient_credit": true, "insufficient_pocket": true,
	"rpc_timeout": true, "rpc_failed": true, "persistence_error": true,
	"already_claimed": true,
	// Exercise 2: add the class you discover here.
}

// Observer switches between the "legacy" logging most services start with
// and the "business" record contract taught in this workshop.
type Observer struct {
	business bool
	logger   *slog.Logger
	legacy   *log.Logger
	registry *prometheus.Registry
	records  *prometheus.CounterVec
	httpDur  *prometheus.HistogramVec
}

func newObserver(mode string) *Observer {
	reg := prometheus.NewRegistry()
	o := &Observer{
		business: mode != "legacy",
		logger:   slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})),
		legacy:   log.New(os.Stdout, "", log.LstdFlags),
		registry: reg,
		records: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "workshop_business_records_total",
			Help: "Business records by operation, stage, outcome, error_class and money_effect.",
		}, []string{"operation", "stage", "outcome", "error_class", "money_effect"}),
		httpDur: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "workshop_http_request_duration_seconds",
			Help:    "HTTP request duration by route and status.",
			Buckets: []float64{.01, .05, .1, .25, .5, 1, 2, 3, 5},
		}, []string{"route", "status"}),
	}
	reg.MustRegister(o.httpDur)
	if o.business {
		reg.MustRegister(o.records)
	}
	return o
}

// Record is one business decision. msg becomes "<operation>_<stage>".
type Record struct {
	Operation   string
	Stage       string
	Outcome     string
	ErrorClass  string
	MoneyEffect string
	Username    string
	Start       time.Time
	Attrs       []slog.Attr
}

// at returns a copy of r for one stage, keeping operation/username/start.
func (r Record) at(stage, outcome, errorClass, money string, attrs ...slog.Attr) Record {
	r.Stage, r.Outcome, r.ErrorClass, r.MoneyEffect = stage, outcome, errorClass, money
	r.Attrs = attrs
	return r
}

// decisionLevel: accepted=INFO, rejected without money movement=WARN, rest=ERROR.
func decisionLevel(outcome, money string) slog.Level {
	switch {
	case outcome == Accepted:
		return slog.LevelInfo
	case outcome == Rejected && money == MoneyNone:
		return slog.LevelWarn
	default:
		return slog.LevelError
	}
}

func (o *Observer) Business(ctx context.Context, r Record) {
	if !o.business {
		return
	}
	if r.ErrorClass == "" {
		r.ErrorClass = "none"
	}
	attrs := []slog.Attr{
		slog.String("trace_id", traceID(ctx)),
		slog.String("span_id", spanID(ctx)),
		slog.String("operation", r.Operation),
		slog.String("stage", r.Stage),
		slog.String("outcome", r.Outcome),
		slog.String("error_class", r.ErrorClass),
		slog.String("money_effect", r.MoneyEffect),
		slog.String("username", r.Username),
		slog.Int64("duration_ms", time.Since(r.Start).Milliseconds()),
	}
	attrs = append(attrs, r.Attrs...)
	level := decisionLevel(r.Outcome, r.MoneyEffect)
	o.logger.LogAttrs(ctx, level, r.Operation+"_"+r.Stage, attrs...)
	recordOnSpan(trace.SpanFromContext(ctx), r.Operation+"_"+r.Stage, level, attrs)

	class := r.ErrorClass
	if !knownErrorClasses[class] {
		class = "other"
	}
	o.records.WithLabelValues(r.Operation, r.Stage, r.Outcome, class, r.MoneyEffect).Inc()
}

// recordOnSpan puts the business record on the current span as an event, so the
// trace view shows the same decision as the log line. ERROR records mark the span red.
func recordOnSpan(span trace.Span, msg string, level slog.Level, attrs []slog.Attr) {
	if !span.IsRecording() {
		return
	}
	kv := make([]attribute.KeyValue, 0, len(attrs))
	for _, a := range attrs {
		if a.Key == "trace_id" || a.Key == "span_id" {
			continue
		}
		kv = append(kv, attribute.String(a.Key, a.Value.String()))
	}
	span.AddEvent(msg, trace.WithAttributes(kv...))
	if level >= slog.LevelError {
		span.SetStatus(codes.Error, msg)
	}
}

// Legacy prints the kind of line older code emits: no trace, no structure.
func (o *Observer) Legacy(format string, args ...any) {
	if o.business {
		return
	}
	o.legacy.Printf(format, args...)
}

func (o *Observer) Startup(role, addr string) {
	if o.business {
		o.logger.Info("service_started", "role", role, "addr", addr, "observe_mode", "business")
		return
	}
	o.legacy.Printf("start server %s", addr)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Middleware starts a server span per request (continuing the caller's trace
// from the W3C traceparent header), returns the trace id in X-Trace-Id, and logs every request.
func (o *Observer) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		ctx := propagation.TraceContext{}.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		ctx, span := tracer.Start(ctx, r.Method+" "+r.URL.Path, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()
		id := traceID(ctx)
		w.Header().Set("X-Trace-Id", id)
		r = r.WithContext(ctx)

		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		elapsed := time.Since(start)

		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		span.SetName(route)
		span.SetAttributes(attribute.String("http.route", route), attribute.Int("http.response.status_code", sw.status))
		if sw.status >= 500 {
			span.SetStatus(codes.Error, fmt.Sprint(sw.status))
		}
		o.httpDur.WithLabelValues(route, fmt.Sprint(sw.status)).Observe(elapsed.Seconds())
		if o.business {
			o.logger.LogAttrs(r.Context(), slog.LevelInfo, "http_request_completed",
				slog.String("trace_id", id),
				slog.String("span_id", spanID(ctx)),
				slog.String("method", r.Method),
				slog.String("route", route),
				slog.Int("status", sw.status),
				slog.Int64("duration_ms", elapsed.Milliseconds()),
			)
			return
		}
		o.legacy.Printf("[HTTP] %3d | %10v | %s %q", sw.status, elapsed.Round(time.Microsecond), r.Method, r.URL.Path)
	})
}
