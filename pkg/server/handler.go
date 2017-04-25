package main

import (
	"html/template"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/matt-deboer/mpp/pkg/selector"
	"github.com/vulcand/oxy/buffer"
	"github.com/vulcand/oxy/forward"
)

type proxyHandler struct {
	selector  *selector.Selector
	rewriter  urlRewriter
	delegate  http.Handler
	endpoints *selector.Result
	started   time.Time
}

type urlRewriter func(url *url.URL)

var clusterStatus, _ = template.New("cluster-status").Parse(clusterStatusTemplate)

func newProxyHandler(s *selector.Selector) *proxyHandler {
	fwd, _ := forward.New()
	b, _ := buffer.New(fwd, buffer.Retry(`IsNetworkError() && Attempts() < 2`))
	proxy := &proxyHandler{
		selector:  s,
		delegate:  b,
		rewriter:  func(u *url.URL) {},
		endpoints: &selector.Result{},
		started:   time.Now(),
	}
	proxy.doSelection()

	return proxy
}

func (p *proxyHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/health" {
		io.WriteString(w, "OK")
	} else if req.URL.Path == "/cluster-status" {
		err := clusterStatus.Execute(w, &templateData{
			SelectionResult:     p.endpoints,
			SelectorStrategy:    p.selector.Strategy.Name(),
			SelectorDescription: p.selector.Strategy.Description(),
			Uptime:              time.Now().Sub(p.started),
			Version:             Version,
			GoVersion:           runtime.Version(),
		})
		if err != nil {
			log.Error(err)
		}
	} else {
		p.rewriter(req.URL)
		if len(p.endpoints.Selection) == 0 {
			http.Error(w, "No backends available", 503)
		} else {
			p.delegate.ServeHTTP(w, req)
		}
	}
}

func (p *proxyHandler) doSelection() {

	result, err := p.selector.Select()
	if err != nil {
		log.Errorf("Current selection will not be updated; selector returned error: %v", err)
	} else {
		if len(result.Selection) == 0 {
			log.Errorf("Current selection will not be updated; selector returned error: %v", err)
		} else if len(result.Selection) == 1 {
			if log.GetLevel() >= log.DebugLevel {
				log.Debugf("Selected targets: %v", result.Selection)
			}
			if !equal(p.endpoints.Selection, result.Selection) {
				log.Infof("New targets differ from current selection %v; updating rewriter => %v", p.endpoints.Selection, result.Selection)
				p.rewriter = func(u *url.URL) {
					u.Scheme = result.Selection[0].Scheme
					u.Host = result.Selection[0].Host
				}
				p.endpoints = result
			} else if log.GetLevel() >= log.DebugLevel {
				log.Debugf("Selection is unchanged: %v", p.endpoints.Selection)
			}
		}
		// TODO: additional handling multiple targets with sticky sessions
	}
}

func equal(a, b []*url.URL) bool {
	for i, v := range a {
		if *v != *b[i] {
			return false
		}
	}
	return len(a) == len(b)
}
