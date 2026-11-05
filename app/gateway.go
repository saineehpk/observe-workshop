package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strings"
)

// runGateway plays the ingress in front of every website. Each website is its
// own namespace with its own api + worker; the gateway picks one by the X-Site
// header (in production that is the website's domain). It does not log or
// trace: like a plain ingress, the story starts at demo-customer-api.
//
//	SITES=web-alpha=http://api-alpha:8080|http://worker-alpha:8081,web-bravo=...
//	/worker/... goes to that website's worker (used by scenarios to read balances)
func runGateway(addr, sites string) {
	type backend struct{ api, worker *httputil.ReverseProxy }
	backends := map[string]backend{}
	var names []string
	for _, entry := range strings.Split(sites, ",") {
		name, urls, ok := strings.Cut(strings.TrimSpace(entry), "=")
		apiURL, workerURL, ok2 := strings.Cut(urls, "|")
		if !ok || !ok2 {
			log.Fatalf("bad SITES entry %q (want name=apiURL|workerURL)", entry)
		}
		a, err1 := url.Parse(apiURL)
		w, err2 := url.Parse(workerURL)
		if err1 != nil || err2 != nil {
			log.Fatalf("bad URL in SITES entry %q", entry)
		}
		backends[name] = backend{httputil.NewSingleHostReverseProxy(a), httputil.NewSingleHostReverseProxy(w)}
		names = append(names, name)
	}
	sort.Strings(names)

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, ok := backends[r.Header.Get("X-Site")]
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"message": "ใส่ header X-Site เป็นชื่อเว็บ", "sites": names})
			return
		}
		if rest, isWorker := strings.CutPrefix(r.URL.Path, "/worker"); isWorker {
			r.URL.Path = rest
			b.worker.ServeHTTP(w, r)
			return
		}
		b.api.ServeHTTP(w, r)
	})
	log.Printf("gateway listening on %s for %v", addr, names)
	log.Fatal(http.ListenAndServe(addr, h))
}
