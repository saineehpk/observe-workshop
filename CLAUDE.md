# Observe Workshop

Local training repo. Everything runs in Docker on the learner's machine with synthetic data.
Reply in the language the learner writes in.

## Where telemetry lives

- MCP server `grafana-local` (created by `make mcp`)
- Loki datasource uid `loki`: labels `service_name` (`demo-customer-api`, `demo-withdraw-worker`) and `namespace="workshop"`
- Prometheus datasource uid `prometheus`: metric `workshop_business_records_total{operation,stage,outcome,error_class,money_effect}` and `workshop_http_request_duration_seconds`
- Tempo datasource uid `tempo`: traces from both services (OpenTelemetry, W3C `traceparent`). Each business record is also a span event.
- Log lines are JSON. The event name is in `msg`. Records join across services by `trace_id`; `span_id` points to the span in Tempo.

## Record contract

Read [docs/record-contract.md](docs/record-contract.md) before proposing or writing records:
closed `outcome` (accepted/rejected/failed/partial), `money_effect` (none/confirmed/unknown),
bounded `error_class`, never rename an existing `msg`, never put usernames or ids in metric labels.

## How to help learners

- Show every LogQL/PromQL query you run so the learner can repeat it in Grafana Explore.
- Label claims Observed / Inferred / Unknown, and cite the log line or query behind each.
- Exercises in [docs/exercises.md](docs/exercises.md) are for the learner to solve. Give hints
  and evidence; do not edit `app/` unless the learner asks you to.
- Grafana annotations tagged `spike` are the answer key for exercise 5. Do not read them
  unless the learner asks to check their answer.
- App code: `app/` (Go, stdlib + prometheus client). Rebuild with `docker compose up -d --build api worker`.
