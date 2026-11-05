package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

// Worker plays the withdraw worker that owns the credit ledger. Like a queue
// consumer, it finishes the job even after the caller has given up.
type Worker struct {
	obs   *Observer
	store *Store
}

type workerResponse struct {
	OK           bool   `json:"ok"`
	Reason       string `json:"reason,omitempty"`
	CreditBefore int64  `json:"credit_before"`
	CreditAfter  int64  `json:"credit_after"`
}

func (wk *Worker) routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /rpc/withdraw", wk.withdraw)
	mux.HandleFunc("GET /state/{username}", stateHandler(wk.store))
}

func (wk *Worker) withdraw(w http.ResponseWriter, r *http.Request) {
	var req playerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, workerResponse{Reason: "bad_request"})
		return
	}
	ctx := context.WithoutCancel(r.Context())
	rec := Record{Operation: "withdraw_worker", Username: req.Username, Start: time.Now()}

	// delay_ms simulates a slow bank / office call.
	time.Sleep(time.Duration(req.DelayMS) * time.Millisecond)

	before, after, err := wk.store.Debit(req.Username, req.Amount)
	callerGone := r.Context().Err() != nil
	var res workerResponse
	switch {
	case errors.Is(err, ErrInsufficient):
		res = workerResponse{Reason: "insufficient_credit", CreditBefore: before, CreditAfter: after}
		wk.obs.Business(ctx, rec.at("completed", Rejected, "insufficient_credit", MoneyNone,
			slog.Int64("amount", req.Amount), slog.Int64("credit_before", before),
			slog.Bool("caller_gone", callerGone)))
		wk.obs.Legacy("withdraw fail")
	case err != nil:
		res = workerResponse{Reason: "unknown_user"}
		wk.obs.Business(ctx, rec.at("completed", Rejected, "unknown_user", MoneyNone))
		wk.obs.Legacy("withdraw fail")
	default:
		res = workerResponse{OK: true, CreditBefore: before, CreditAfter: after}
		wk.obs.Business(ctx, rec.at("completed", Accepted, "", MoneyConfirmed,
			slog.Int64("amount", req.Amount),
			slog.Int64("credit_before", before), slog.Int64("credit_after", after),
			slog.Bool("caller_gone", callerGone)))
		wk.obs.Legacy("withdraw success")
	}
	writeJSON(w, http.StatusOK, res)
}
