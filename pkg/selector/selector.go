package selector

import (
	"fmt"
	"net/url"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/matt-deboer/mpp/pkg/locator"
)

// Selector is a puggable interface for viable prometheus endpoints
type Selector struct {
	locators []locator.Locator
	Strategy Strategy
}

// Result encapsulates a selection result, including the candidate endpoints considered
type Result struct {
	Selection  []*url.URL
	Candidates []*locator.PrometheusEndpoint
}

func (r *Result) String() string {
	return fmt.Sprintf("%v", r.Selection)
}

// NewSelector returns a new Selector instance of the specified type
func NewSelector(strategyName string, locators []locator.Locator) (*Selector, error) {
	if strategy, ok := strategies[strategyName]; ok {
		return &Selector{locators: locators, Strategy: strategy}, nil
	}
	return nil, fmt.Errorf("No selector strategy named '%s' found", strategyName)
}

// Select performs selection of a/all viable prometheus endpoint target(s)
func (s *Selector) Select() (result *Result, err error) {

	endpoints := make([]*locator.PrometheusEndpoint, 0, 3)
	for _, loc := range s.locators {
		clients, err := loc.Endpoints()
		if err != nil {
			if clients != nil && len(clients) > 0 {
				endpoints = append(endpoints, clients...)
				log.Errorf("Locator %v resolved the following endpoints: %v, with errors: %v", loc, clients, err)
			} else {
				log.Errorf("Locator %v failed to resolve endpoints: %v", loc, err)
			}
		} else {
			endpoints = append(endpoints, clients...)
			if log.GetLevel() >= log.DebugLevel {
				log.Debugf("Locator %v resolved the following endpoints: %v", loc, clients)
			}
		}
	}
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("No endpoints returned by any locators")
	}

	result = &Result{
		Candidates: endpoints,
	}

	err = s.Strategy.Select(endpoints)
	if err != nil {
		return result, err
	}

	result.Selection = make([]*url.URL, 0, len(endpoints))
	for _, endpoint := range endpoints {
		if endpoint.Selected {
			target, err := url.ParseRequestURI(endpoint.Address)
			if err != nil {
				log.Errorf("Failed to parse selection '%s': %v", endpoint, err)
			} else {
				result.Selection = append(result.Selection, target)
			}
		}
	}
	return result, nil
}

// Strategy is a puggable interface for strategies to select viable prometheus endpoints
type Strategy interface {
	// Select chooses elligible prometheus endpoints out of the provided set, marking the 'Selected' flag on the chosen items
	Select(endpoints []*locator.PrometheusEndpoint) (err error)
	// Name provides the (unique) name of this strategy
	Name() string
	// Description provides a human-readable description for this strategy
	Description() string
	// ComparisonMetricName gets the name of the comparison metric/calculation used to make a selection
	ComparisonMetricName() string
}

// All registered platforms
var (
	strategyMutex sync.Mutex
	strategies    = make(map[string]Strategy)
)

// RegisterStrategy registers a selector strategy
func RegisterStrategy(name string, strategy Strategy) {
	strategyMutex.Lock()
	defer strategyMutex.Unlock()
	strategies[name] = strategy
	if log.GetLevel() >= log.DebugLevel {
		log.Debugf("Registered strategy '%s': %v", name, strategy)
	}
}
