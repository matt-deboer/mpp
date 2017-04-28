package main

import (
	"html/template"
	"io"
	"net/http"
	"runtime"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/matt-deboer/mpp/pkg/router"
	"github.com/matt-deboer/mpp/pkg/version"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type mppHandler struct {
	started time.Time
	router  *router.Router
	prom    http.Handler
}

// Namespace is the common namespace shared by metrics, url paths, etc. for this app
const Namespace = "mpp"

var clusterStatus, _ = template.New("cluster-status").Parse(clusterStatusTemplate)

func newMPPHandler(r *router.Router) *mppHandler {

	buildInfo := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "build_info",
		Help:      "The number of currently selected backends",
		ConstLabels: prometheus.Labels{
			"branch":    version.Branch,
			"goversion": runtime.Version(),
			"version":   version.Version,
			"revision":  version.Revision,
		},
	})
	buildInfo.Set(1)
	prometheus.MustRegister(buildInfo)

	return &mppHandler{
		router:  r,
		prom:    promhttp.Handler(),
		started: time.Now(),
	}
}

func (p *mppHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/mpp/health" {
		io.WriteString(w, "OK")
	} else if req.URL.Path == "/mpp/metrics" {
		p.prom.ServeHTTP(w, req)
	} else if req.URL.Path == "/mpp/status" {
		data := &templateData{
			RouterStatus: p.router.Status(),
			Uptime:       time.Now().Sub(p.started),
			Version:      version.Version,
			GoVersion:    runtime.Version(),
		}
		if log.GetLevel() >= log.DebugLevel {
			log.Debugf("Template data: %v", data)
		}
		err := clusterStatus.Execute(w, data)
		if err != nil {
			log.Error(err)
		}
	} else {
		p.router.ServeHTTP(w, req)
	}
}
