package router

import (
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

// internalRouter controls the actual low-level
type internalRouter struct {
	router   *Router
	affinity *affinityProvider
}

func (i *internalRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	handleRetry(req, i.router)

	if len(i.router.selection.Selection) == 0 {
		http.Error(w, "No backends available :(", 503)
	} else {
		target := i.affinity.preferredTarget(req, i.router)
		needsCookie := (i.affinity.cookiesEnabled && target == nil)

		if target != nil {
			if log.GetLevel() >= log.DebugLevel {
				log.Debugf("Router is reusing sticky session to %v", target)
			}
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
		} else {
			if log.GetLevel() >= log.DebugLevel {
				log.Debugf("Router is selecting a new backend target")
			}
			i.router.rewriter(req.URL)
		}
		backend := backend(req.URL)
		i.router.metrics.requestsByBackend.WithLabelValues(backend).Inc()

		w.Header().Set("MPP.ServedBy", backend)
		start := time.Now()
		i.router.forward.ServeHTTP(w, req)
		i.router.metrics.responseTimeByBackend.WithLabelValues(backend).Add(time.Now().Sub(start).Seconds() * 1000)
		i.affinity.savePreferredTarget(w, req, needsCookie)
	}
}
