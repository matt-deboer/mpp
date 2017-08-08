package router

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"sync"

	"github.com/matt-deboer/mpp/pkg/locator"
	"github.com/matt-deboer/mpp/pkg/selector"
	"github.com/matt-deboer/mpp/pkg/version"
	log "github.com/sirupsen/logrus"
	"github.com/vulcand/oxy/buffer"
	"github.com/vulcand/oxy/forward"
)

// Router provides dynamic routing of http requests based on a configurable strategy
type Router struct {
	locators        []locator.Locator
	selector        *selector.Selector
	selection       *selector.Result
	forward         http.Handler
	buffer          *buffer.Buffer
	rewriter        urlRewriter
	affinityOptions []AffinityOption
	interval        time.Duration
	metrics         *metrics
	// used to mark control of the selection process
	theConch            chan struct{}
	selectionInProgress sync.RWMutex
	shutdownHook        chan struct{}
}

// Status contains a snapshot status summary of the router state
type Status struct {
	Endpoints           []*locator.PrometheusEndpoint
	Strategy            string
	StrategyDescription string
	AffinityOptions     string
	ComparisonMetric    string
	Interval            time.Duration
}

type urlRewriter func(u *url.URL)

var noOpRewriter = func(u *url.URL) {}

// NewRouter constructs a new router based on the provided stategy and locators
func NewRouter(interval time.Duration, affinityOptions []AffinityOption,
	locators []locator.Locator, strategyArgs ...string) (*Router, error) {

	sel, err := selector.NewSelector(locators, strategyArgs...)
	if err != nil {
		return nil, err
	}

	r := &Router{
		locators:        locators,
		selector:        sel,
		affinityOptions: affinityOptions,
		interval:        interval,
		rewriter:        noOpRewriter,
		metrics:         newMetrics(version.Name),
		selection:       &selector.Result{},
		theConch:        make(chan struct{}, 1),
		shutdownHook:    make(chan struct{}, 1),
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

			select {
			case _ = <-r.shutdownHook:
				return
			default:
				r.doSelection()
			}
		}
	}()

	r.forward, _ = forward.New()
	r.buffer, _ = buffer.New(&internalRouter{
		router:   r,
		affinity: newAffinityProvider(affinityOptions),
	},
		buffer.Retry(`IsNetworkError() && Attempts() < 2`))
	return r, nil
}

// Close stops the router's background selection routine
func (r *Router) Close() {
	r.shutdownHook <- struct{}{}
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.buffer.ServeHTTP(w, retryableRequest(req))
}

func (r *Router) doSelection() {
	select {
	case _ = <-r.theConch:
		r.selectionInProgress.Lock()
		defer func() {
			r.selectionInProgress.Unlock()
			if log.GetLevel() >= log.DebugLevel {
				log.Debugf("Returning selection lock")
			}
			r.theConch <- struct{}{}
		}()

		if log.GetLevel() >= log.DebugLevel {
			log.Debugf("Got selection lock; performing selection")
		}

		result, err := r.selector.Select()

		if len(result.Selection) == 0 {
			if err != nil {
				log.Errorf("Selector returned no valid selection, and error: %v", err)
				if r.selection == nil {
					r.selection = result
					r.rewriter = noOpRewriter
				}
			} else {
				r.selection = result
				r.rewriter = noOpRewriter
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
			} else {
				log.Infof("Selection is unchanged: %v, out of candidates: %v", r.selection.Selection, r.selection.Candidates)
			}
			r.selection = result
		}

		r.metrics.selectedBackends.Set(float64(len(result.Selection)))
		r.metrics.selectionEvents.Inc()

	default:
		log.Warnf("Selection is already in-progress; awaiting result")
		r.selectionInProgress.RLock()
		r.selectionInProgress.RUnlock()
	}
}

func equal(a, b []*url.URL) bool {
	if len(a) == len(b) {
		for i, v := range a {
			if *v != *b[i] {
				return false
			}
		}
		return true
	}
	return false
}

func contains(a []*url.URL, u *url.URL) bool {
	for _, v := range a {
		if *u == *v {
			return true
		}
	}
	return false
}

func backend(u *url.URL) string {
	return fmt.Sprintf("%s://%s", u.Scheme, u.Host)
}

// Status returns a summary of the router's current state
func (r *Router) Status() *Status {
	return &Status{
		Endpoints:           r.selection.Candidates,
		Strategy:            r.selector.Strategy.Name(),
		StrategyDescription: r.selector.Strategy.Description(),
		ComparisonMetric:    r.selector.Strategy.ComparisonMetricName(),
		AffinityOptions:     strings.Trim(fmt.Sprintf("%v", r.affinityOptions), "[]"),
		Interval:            r.interval,
	}
}
