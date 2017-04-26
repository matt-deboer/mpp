package router

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/matt-deboer/mpp/pkg/locator"
	"github.com/matt-deboer/mpp/pkg/selector"
	"github.com/vulcand/oxy/buffer"
	"github.com/vulcand/oxy/forward"
)

const cookieName = "MMP.Route"

// Router provides dynamic routing of http requests based on a configurable strategy
type Router struct {
	locators            []locator.Locator
	selector            *selector.Selector
	selection           *selector.Result
	forward             http.Handler
	buffer              *buffer.Buffer
	rewriter            urlRewriter
	sticky              bool
	interval            time.Duration
	theConch            chan struct{}
	selectionInProgress sync.RWMutex
}

// Status contains a snapshot status summary of the router state
type Status struct {
	Endpoints           []*locator.PrometheusEndpoint
	Strategy            string
	StrategyDescription string
	Sticky              bool
	ComparisonMetric    string
	Interval            time.Duration
}

type contextKey int

var retryKey contextKey

type urlRewriter func(u *url.URL)

// NewRouter constructs a new router based on the provided stategy and locators
func NewRouter(strategy string, interval time.Duration, locators []locator.Locator) (*Router, error) {
	sel, err := selector.NewSelector(strategy, locators)
	if err != nil {
		return nil, err
	}
	r := &Router{
		locators: locators,
		selector: sel,
		sticky:   sel.Strategy.RequiresStickySessions(),
		interval: interval,
		theConch: make(chan struct{}, 1),
	}
	// Set up the lock
	r.theConch <- struct{}{}
	r.doSelection()
	go func() {
		// TODO: create shutdown channel for this
		for {
			if log.GetLevel() >= log.DebugLevel {
				log.Debugf("Backend selection is sleeping for %s", interval)
			}
			time.Sleep(r.interval)
			r.doSelection()
		}
	}()

	r.forward, _ = forward.New()
	r.buffer, _ = buffer.New(&internalRouter{r}, buffer.Retry(`IsNetworkError() && Attempts() < 2`))
	return r, nil
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.buffer.ServeHTTP(w,
		req.WithContext(context.WithValue(req.Context(), retryKey, &retry{})))
}

type internalRouter struct {
	router *Router
}

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

func (i *internalRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var target *url.URL
	var needsCookie = false

	if shouldRetry(req) {
		log.Warnf("Backend selection forced by retry")
		i.router.doSelection()
	} else {
		retryOnNext(req)
	}

	if i.router.sticky {
		needsCookie = true
		cookie, _ := req.Cookie(cookieName)
		if cookie != nil {
			u, err := url.Parse(cookie.Value)
			if err != nil {
				log.Errorf("Sticky cookie contained unparsable url %s: %v", cookie.Value, err)
			} else if !contains(i.router.selection.Selection, u) {
				if log.GetLevel() >= log.DebugLevel {
					log.Debugf("Sticky cookie target %v is no longer valid", u)
				}
			} else {
				target = u
				needsCookie = false
			}
		}
	}
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

	if needsCookie {
		http.SetCookie(w, &http.Cookie{
			Name:     cookieName,
			Value:    fmt.Sprintf("%s://%s", req.URL.Scheme, req.URL.Host),
			HttpOnly: true,
		})
	}
	w.Header().Set("MPP.ServedBy", fmt.Sprintf("%s://%s", req.URL.Scheme, req.URL.Host))
	i.router.forward.ServeHTTP(w, req)
}

func (r *Router) doSelection() {
	select {
	case _ = <-r.theConch:
		r.selectionInProgress.Lock()
		defer r.selectionInProgress.Unlock()
		if log.GetLevel() >= log.DebugLevel {
			log.Debugf("Got selection lock; performing selection")
		}

		result, err := r.selector.Select()
		if err != nil {
			if result != nil {
				log.Errorf("Current selection is updated, with error: %v", err)
			} else {
				log.Errorf("Current selection will not be updated; selector returned no restult, and error: %v", err)
			}
		}

		if result.Selection == nil || len(result.Selection) == 0 {
			if err != nil {
				log.Errorf("Current selection will not be updated; selector returned no restult, and error: %v", err)
			} else {
				r.selection = result
				log.Warnf("Selector returned no valid selection")
			}
		} else {
			if log.GetLevel() >= log.DebugLevel {
				log.Debugf("Selected targets: %v", result.Selection)
			}
			if r.selection == nil || !equal(r.selection.Selection, result.Selection) {
				log.Infof("New targets differ from current selection %v; updating rewriter => %v", r.selection, result)
				r.rewriter = func(u *url.URL) {
					selection := result.Selection
					i := r.selector.Strategy.NextIndex(selection)
					target := selection[i]
					u.Host = target.Host
					u.Scheme = target.Scheme
				}
			} else if log.GetLevel() >= log.DebugLevel {
				log.Debugf("Selection is unchanged: %v", r.selection)
			}
			r.selection = result
		}

		if log.GetLevel() >= log.DebugLevel {
			log.Debugf("Returning selection lock")
		}
		r.theConch <- struct{}{}
	default:
		if log.GetLevel() >= log.DebugLevel {
			log.Debugf("Selection is already in-progress; awaiting result")
		}
		r.selectionInProgress.RLock()
		defer r.selectionInProgress.RUnlock()
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

func contains(a []*url.URL, u *url.URL) bool {
	for _, v := range a {
		if *u == *v {
			return true
		}
	}
	return false
}

// Status returns a summary of the router's current state
func (r *Router) Status() *Status {
	return &Status{
		Endpoints:           r.selection.Candidates,
		Strategy:            r.selector.Strategy.Name(),
		StrategyDescription: r.selector.Strategy.Description(),
		ComparisonMetric:    r.selector.Strategy.ComparisonMetricName(),
		Sticky:              r.selector.Strategy.RequiresStickySessions(),
		Interval:            r.interval,
	}
}
