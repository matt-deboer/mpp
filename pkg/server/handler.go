package main

import (
	"net/http"
	"net/url"

	log "github.com/Sirupsen/logrus"
	"github.com/matt-deboer/mpp/pkg/selector"
	"github.com/vulcand/oxy/buffer"
	"github.com/vulcand/oxy/forward"
)

type proxyHandler struct {
	selector  *selector.Selector
	rewriter  urlRewriter
	delegate  http.Handler
	selection []*url.URL
}

type urlRewriter func(url *url.URL)

func newProxyHandler(s *selector.Selector) *proxyHandler {
	fwd, _ := forward.New()
	b, _ := buffer.New(fwd, buffer.Retry(`IsNetworkError() && Attempts() < 2`))
	proxy := &proxyHandler{selector: s, delegate: b}
	proxy.doSelection()
	return proxy
}

func (p *proxyHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	p.rewriter(req.URL)
	p.delegate.ServeHTTP(w, req)
}

func (p *proxyHandler) doSelection() {

	targets, err := p.selector.Select()
	if err != nil {
		log.Errorf("Current selection will not be updated; selector returned error: %v", err)
	} else {
		if len(targets) == 0 {
			log.Errorf("Current selection will not be updated; selector returned error: %v", err)
		} else if len(targets) == 1 {
			if log.GetLevel() >= log.DebugLevel {
				log.Debugf("Selected targets: %v", targets)
			}
			if !equal(p.selection, targets) {
				log.Infof("New targets differ from current selection %v; updating rewriter => %v", p.selection, targets)
				p.rewriter = func(u *url.URL) {
					u.Scheme = targets[0].Scheme
					u.Host = targets[0].Host
				}
				p.selection = targets
			} else if log.GetLevel() >= log.DebugLevel {
				log.Debugf("Selection is unchanged: %v", p.selection)
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
