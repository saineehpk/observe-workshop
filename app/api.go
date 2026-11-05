package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// API plays the player-facing service: withdraw (via RPC to the worker),
// pocket exchange, and daily check-in.
type API struct {
	obs           *Observer
	store         *Store
	workerURL     string
	rpcTimeout    time.Duration
	consumerDelay time.Duration
	client        *http.Client
	statements    chan statementEvent
}

type statementEvent struct {
	traceID  string
	username string
}

type playerRequest struct {
	Username string `json:"username"`
	Amount   int64  `json:"amount"`
	DelayMS  int    `json:"delay_ms"`
}

const checkinReward = 10

func newAPI(obs *Observer, store *Store, workerURL string, rpcTimeout, consumerDelay time.Duration) *API {
	a := &API{
		obs:           obs,
		store:         store,
		workerURL:     workerURL,
		rpcTimeout:    rpcTimeout,
		consumerDelay: consumerDelay,
		client:        &http.Client{},
		statements:    make(chan statementEvent, 256),
	}
	go a.consumeStatements()
	return a
}

func (a *API) routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /withdraw", a.withdraw)
	mux.HandleFunc("POST /pocket/exchange", a.pocketExchange)
	mux.HandleFunc("POST /checkin/claim", a.checkinClaim)
	mux.HandleFunc("GET /state/{username}", stateHandler(a.store))
}

func (a *API) withdraw(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req, ok := decode(w, r)
	if !ok {
		return
	}
	rec := Record{Operation: "withdraw", Username: req.Username, Start: time.Now()}
	a.obs.Business(ctx, rec.at("requested", Accepted, "", MoneyNone, slog.Int64("amount", req.Amount)))

	if _, err := a.store.Get(req.Username); err != nil {
		a.obs.Business(ctx, rec.at("completed", Rejected, "unknown_user", MoneyNone))
		a.obs.Legacy("withdraw error: %v", err)
		reply(w, ctx, http.StatusNotFound, "ไม่พบผู้ใช้")
		return
	}
	if req.Amount <= 0 {
		a.obs.Business(ctx, rec.at("completed", Rejected, "invalid_amount", MoneyNone, slog.Int64("amount", req.Amount)))
		reply(w, ctx, http.StatusBadRequest, "ยอดถอนไม่ถูกต้อง")
		return
	}

	rpcCtx, cancel := context.WithTimeout(ctx, a.rpcTimeout)
	defer cancel()
	res, err := a.callWorker(rpcCtx, req)
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		// The worker may still debit after we give up: money_effect is unknown.
		a.obs.Business(ctx, rec.at("completed", Failed, "rpc_timeout", MoneyUnknown,
			slog.Int64("amount", req.Amount),
			slog.Int64("rpc_timeout_ms", a.rpcTimeout.Milliseconds())))
		a.obs.Legacy("withdraw rpc error: %v", err)
		reply(w, ctx, http.StatusAccepted, "รายการกำลังดำเนินการ กรุณาตรวจสอบประวัติการถอน")
	case err != nil:
		a.obs.Business(ctx, rec.at("completed", Failed, "rpc_failed", MoneyUnknown, slog.String("error", err.Error())))
		a.obs.Legacy("withdraw rpc error: %v", err)
		reply(w, ctx, http.StatusBadGateway, "ระบบขัดข้อง กรุณาลองใหม่")
	case !res.OK:
		a.obs.Business(ctx, rec.at("completed", Rejected, res.Reason, MoneyNone, slog.Int64("amount", req.Amount)))
		a.obs.Legacy("withdraw fail")
		reply(w, ctx, http.StatusUnprocessableEntity, "ยอดเงินไม่พอ")
	default:
		a.obs.Business(ctx, rec.at("completed", Accepted, "", MoneyConfirmed,
			slog.Int64("amount", req.Amount),
			slog.Int64("credit_before", res.CreditBefore),
			slog.Int64("credit_after", res.CreditAfter)))
		reply(w, ctx, http.StatusOK, "ถอนสำเร็จ")
	}
}

func (a *API) callWorker(ctx context.Context, req playerRequest) (workerResponse, error) {
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.workerURL+"/rpc/withdraw", bytes.NewReader(body))
	if err != nil {
		return workerResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Trace-Id", traceID(ctx))
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return workerResponse{}, err
	}
	defer resp.Body.Close()
	var out workerResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return workerResponse{}, fmt.Errorf("decode worker response: %w", err)
	}
	return out, nil
}

func (a *API) pocketExchange(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req, ok := decode(w, r)
	if !ok {
		return
	}
	rec := Record{Operation: "pocket_exchange", Username: req.Username, Start: time.Now()}
	if req.Amount <= 0 {
		a.obs.Business(ctx, rec.at("completed", Rejected, "invalid_amount", MoneyNone, slog.Int64("amount", req.Amount)))
		reply(w, ctx, http.StatusBadRequest, "ยอดไม่ถูกต้อง")
		return
	}

	before, after, err := a.store.ExchangePocket(req.Username, req.Amount, time.Now())
	switch {
	case errors.Is(err, ErrUnknownUser):
		a.obs.Business(ctx, rec.at("completed", Rejected, "unknown_user", MoneyNone))
		reply(w, ctx, http.StatusNotFound, "ไม่พบผู้ใช้")
	case errors.Is(err, ErrInsufficient):
		a.obs.Business(ctx, rec.at("completed", Rejected, "insufficient_pocket", MoneyNone,
			slog.Int64("amount", req.Amount), slog.Int64("pocket_before", before.Pocket)))
		reply(w, ctx, http.StatusUnprocessableEntity, "ยอดในกระเป๋าไม่พอ")
	case err != nil:
		// Exercise 2: this is what the first version of the record looked like.
		// Is "persistence_error / unknown" really what happened here?
		a.obs.Business(ctx, rec.at("completed", Failed, "persistence_error", MoneyUnknown,
			slog.Int64("amount", req.Amount),
			slog.String("error", err.Error())))
		a.obs.Legacy("pocket exchange error: %v", err)
		reply(w, ctx, http.StatusInternalServerError, "เกิดข้อผิดพลาด กรุณาลองใหม่")
	default:
		a.obs.Business(ctx, rec.at("completed", Accepted, "", MoneyConfirmed,
			slog.Int64("amount", req.Amount),
			slog.Int64("pocket_before", before.Pocket), slog.Int64("pocket_after", after.Pocket),
			slog.Int64("credit_before", before.Credit), slog.Int64("credit_after", after.Credit)))
		reply(w, ctx, http.StatusOK, "ย้ายเข้าเครดิตสำเร็จ")
	}
}

func (a *API) checkinClaim(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req, ok := decode(w, r)
	if !ok {
		return
	}
	rec := Record{Operation: "checkin_claim", Username: req.Username, Start: time.Now()}
	before, err := a.store.ClaimCheckin(req.Username, checkinReward)
	switch {
	case errors.Is(err, ErrUnknownUser):
		a.obs.Business(ctx, rec.at("completed", Rejected, "unknown_user", MoneyNone))
		reply(w, ctx, http.StatusNotFound, "ไม่พบผู้ใช้")
	case errors.Is(err, errAlreadyClaimed):
		a.obs.Business(ctx, rec.at("completed", Rejected, "already_claimed", MoneyNone, slog.Int("stack_before", before.Stack)))
		reply(w, ctx, http.StatusConflict, "วันนี้รับไปแล้ว")
	default:
		a.obs.Business(ctx, rec.at("completed", Accepted, "", MoneyConfirmed,
			slog.Int("stack_before", before.Stack), slog.Int("stack_after", -1),
			slog.Int64("reward", checkinReward)))
		a.obs.Legacy("checkin ok")
		// Creating the reward statement triggers a downstream consumer.
		a.statements <- statementEvent{traceID: traceID(ctx), username: req.Username}
		reply(w, ctx, http.StatusOK, "รับรางวัลเช็คอินสำเร็จ")
	}
}

// consumeStatements plays a separate consumer that refreshes the member's
// stack after a statement is created. It reads from the secondary copy.
func (a *API) consumeStatements() {
	for ev := range a.statements {
		time.Sleep(a.consumerDelay)
		ctx := withTrace(context.Background(), ev.traceID)
		start := time.Now()
		read := a.store.ReadStackSecondary(ev.username)
		previous := a.store.SetStack(ev.username, read)
		rec := Record{Operation: "checkin_stack", Username: ev.username, Start: start}
		a.obs.Business(ctx, rec.at("written", Accepted, "", MoneyNone,
			slog.String("component", "statement-consumer"),
			slog.String("read_source", "secondary"),
			slog.Int("stack_read", read),
			slog.Int("stack_primary_before", previous),
			slog.Int("stack_written", read)))
	}
}

func decode(w http.ResponseWriter, r *http.Request) (playerRequest, bool) {
	var req playerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" {
		reply(w, r.Context(), http.StatusBadRequest, "คำขอไม่ถูกต้อง")
		return req, false
	}
	return req, true
}

func reply(w http.ResponseWriter, ctx context.Context, status int, message string) {
	writeJSON(w, status, map[string]any{"message": message, "trace_id": traceID(ctx)})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func stateHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, err := store.Get(r.PathValue("username"))
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, acc)
	}
}
