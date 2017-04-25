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
	strategy Strategy
}

// NewSelector returns a new Selector instance of the specified type
func NewSelector(strategyName string, locators []locator.Locator) (*Selector, error) {
	if strategy, ok := strategies[strategyName]; ok {
		return &Selector{locators: locators, strategy: strategy}, nil
	}
	return nil, fmt.Errorf("No selector strategy named '%s' found", strategyName)
}

// Select performs selection of a/all viable prometheus endpoint target(s)
func (s *Selector) Select() (targets []*url.URL, err error) {

	endpoints := make([]*locator.PrometheusEndpoint, 0, 3)
	for _, loc := range s.locators {
		clients, err := loc.Endpoints()
		if err != nil {
			log.Errorf("Locator %v failed to resolve endpoints: %v", loc, err)
		} else {
			if log.GetLevel() >= log.DebugLevel {
				log.Debugf("Locator %v resolved the following endpoints: %v", loc, clients)
			}
			endpoints = append(endpoints, clients...)
		}
	}
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("No endpoints returned by any locators")
	}

	selections, err := s.strategy.Select(endpoints)
	if err != nil {
		return nil, err
	}
	targets = make([]*url.URL, 0, len(selections))
	for _, selection := range selections {
		target, err := url.ParseRequestURI(selection)
		if err != nil {
			log.Errorf("Failed to parse selection '%s': %v", selection, err)
		} else {
			targets = append(targets, target)
		}
	}
	return targets, nil
}

// Strategy is a puggable interface for strategies to select viable prometheus endpoints
type Strategy interface {
	// Select chooses elligible prometheus endpoints out of the provided set
	Select(endpoints []*locator.PrometheusEndpoint) (targets []string, err error)
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
