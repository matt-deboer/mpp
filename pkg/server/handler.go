package main

import (
	"html/template"
	"io"
	"net/http"
	"runtime"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/matt-deboer/mpp/pkg/router"
)

type mppHandler struct {
	started time.Time
	router  *router.Router
}

var clusterStatus, _ = template.New("cluster-status").Parse(clusterStatusTemplate)

func newMPPHandler(r *router.Router) *mppHandler {
	return &mppHandler{
		router:  r,
		started: time.Now(),
	}
}

func (p *mppHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/health" {
		io.WriteString(w, "OK")
	} else if req.URL.Path == "/cluster-status" {
		data := &templateData{
			RouterStatus: p.router.Status(),
			Uptime:       time.Now().Sub(p.started),
			Version:      Version,
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
