package router

import (
	"context"
	"net/http"

	"github.com/prometheus/common/log"
)

type contextKey int

var retryKey contextKey

type retry struct {
	value bool
}

func retryOnNext(req *http.Request) {
	if retry, ok := req.Context().Value(retryKey).(*retry); ok {
		retry.value = true
	}
}

func shouldRetry(req *http.Request) bool {
	retry, ok := req.Context().Value(retryKey).(*retry)
	return ok && retry.value
}

func handleRetry(req *http.Request, router *Router) {
	if shouldRetry(req) {
		log.Warnf("Backend selection forced by retry")
		router.metrics.retriesByBackend.WithLabelValues(backend(req.URL)).Inc()
		router.doSelection()
	} else {
		retryOnNext(req)
	}
}

func retryableRequest(req *http.Request) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), retryKey, &retry{}))
}
