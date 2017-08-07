package selector

import (
	"fmt"
	"net/url"
	"sync"

	"github.com/matt-deboer/mpp/pkg/locator"
	log "github.com/sirupsen/logrus"
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
func NewSelector(locators []locator.Locator, strategyArgs ...string) (*Selector, error) {
	if len(strategyArgs) == 0 {
		return nil, fmt.Errorf("At minimum, a strategy name (first arg) must be provided")
	}
	if stratgyFactory, ok := strategies[strategyArgs[0]]; ok {
		strategy, err := stratgyFactory(strategyArgs[1:]...)
		if err != nil {
			return nil, err
		}
		return &Selector{locators: locators, Strategy: strategy}, nil
	}
	return nil, fmt.Errorf("No selector strategy named '%s' found", strategyArgs[0])
}

// Select performs selection of a/all viable prometheus endpoint target(s)
func (s *Selector) Select() (result *Result, err error) {

	result = &Result{
		Candidates: make([]*locator.PrometheusEndpoint, 0, 3),
	}

	for _, loc := range s.locators {
		endpoints, err := loc.Endpoints()
		if err != nil {
			if endpoints != nil && len(endpoints) > 0 {
				result.Candidates = append(result.Candidates, endpoints...)
				log.Warnf("Locator %v resolved the following endpoints: %v, with errors: %v", loc, endpoints, err)
			} else {
				log.Errorf("Locator %v failed to resolve endpoints: %v", loc, err)
			}
		} else {
			result.Candidates = append(result.Candidates, endpoints...)
			if log.GetLevel() >= log.DebugLevel {
				log.Debugf("Locator %v resolved the following endpoints: %v", loc, endpoints)
			}
		}
	}

	if len(result.Candidates) == 0 {
		return result, fmt.Errorf("No endpoints returned by any locators")
	}

	err = s.Strategy.Select(result.Candidates)
	if err != nil {
		return result, err
	}

	result.Selection = make([]*url.URL, 0, len(result.Candidates))
	for _, endpoint := range result.Candidates {
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
	// Select chooses elligible prometheus endpoints out of the provided set, marking
	// the 'Selected' flag on the chosen items, and optionally, setting the 'Error' flag
	// on items that cannot be evaluated
	Select(endpoints []*locator.PrometheusEndpoint) (err error)
	// Name provides the (unique) name of this strategy
	Name() string
	// Description provides a human-readable description for this strategy
	Description() string
	// ComparisonMetricName gets the name of the comparison metric/calculation used to make a selection
	ComparisonMetricName() string
	// NextIndex returns the index of the target that should be used to field the next request
	NextIndex(targets []*url.URL) int
}

// All registered platforms
var (
	strategyMutex sync.Mutex
	strategies    = make(map[string]func(args ...string) (Strategy, error))
)

// RegisterStrategy registers a selector strategy
func RegisterStrategy(name string, factory func(args ...string) (Strategy, error)) {
	strategyMutex.Lock()
	defer strategyMutex.Unlock()
	strategies[name] = factory
	if log.GetLevel() >= log.DebugLevel {
		log.Debugf("Registered strategy '%s'", name)
	}
}
