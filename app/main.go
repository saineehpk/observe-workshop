package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	role := env("ROLE", "api")
	if role == "gateway" {
		runGateway(":"+env("PORT", "8080"), env("SITES", ""))
		return
	}
	obs := newObserver(env("OBSERVE_MODE", "business"))
	defer setupTracing(env("SERVICE_NAME", "demo-"+role), obs.business)(context.Background())
	// users are named after the website: NAMESPACE web-bravo -> bravo0001..bravo0050
	store := newStore(env("USER_PREFIX", strings.TrimPrefix(env("NAMESPACE", "web-player"), "web-")), 50, ms("REPLICA_LAG_MS", 500))

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(obs.registry, promhttp.HandlerOpts{}))
	switch role {
	case "api":
		newAPI(obs, store, env("WORKER_URL", "http://worker:8081"), ms("RPC_TIMEOUT_MS", 2000), ms("CONSUMER_DELAY_MS", 20)).routes(mux)
	case "worker":
		(&Worker{obs: obs, store: store}).routes(mux)
	default:
		log.Fatalf("unknown ROLE %q (want api, worker or gateway)", role)
	}

	addr := ":" + env("PORT", "8080")
	obs.Startup(role, addr)
	log.Fatal(http.ListenAndServe(addr, obs.Middleware(mux)))
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func ms(key string, fallback int) time.Duration {
	n, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		n = fallback
	}
	return time.Duration(n) * time.Millisecond
}
