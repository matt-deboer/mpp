package minimumhistorysticky

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/matt-deboer/mpp/pkg/locator"
	"github.com/matt-deboer/mpp/pkg/selector"
	"github.com/prometheus/common/model"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
	s := &Selector{}
	selector.RegisterStrategy(s.Name(), func(args ...string) (selector.Strategy, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("Strategy %s requires a {minimum-duration} argument", s.Name())
		}
		duration, err := time.ParseDuration(args[0])
		if err != nil {
			return nil, fmt.Errorf("Invalid minimum duration value '%s' for %s: %v", args[0], s.Name(), err)
		}
		return &Selector{minimumHistory: duration}, nil
	})
}

// Selector implements selction of a single prometheus endpoint out of a provided set of endpoints
// which has the highest value of total ingested samples
type Selector struct {
	minimumHistory time.Duration
}

// Name provides the (unique) name of this strategy
func (s *Selector) Name() string {
	return "minimum-history-sticky"
}

// Description provides a human-readable description for this strategy
func (s *Selector) Description() string {
	return "Selects a prometheus instance at random, with sticky sessions"
}

// ComparisonMetricName gets the name of the comparison metric/calculation used to make a selection
func (s *Selector) ComparisonMetricName() string {
	return "prometheus_build_info"
}

// RequiresStickySessions answers whether this strategy needs sticky sessions
func (s *Selector) RequiresStickySessions() bool {
	return true
}

// NextIndex returns the index of the target that should be used to field the next request
func (s *Selector) NextIndex(targets []*url.URL) int {
	next := rand.Intn(len(targets))
	if log.GetLevel() >= log.DebugLevel {
		log.Debugf("Strategy %T returned next index: %d", s, next)
	}
	return next
}

// Select chooses elligible prometheus endpoints out of the provided set
func (s *Selector) Select(endpoints []*locator.PrometheusEndpoint) (err error) {
	selected := 0
	for _, endpoint := range endpoints {
		endpoint.Selected = false
		if endpoint.QueryAPI != nil {
			value, err := endpoint.QueryAPI.Query(context.TODO(), "prometheus_build_info", time.Now().Add(-1*s.minimumHistory))
			if err != nil {
				log.Errorf("Endpoint %v returned error: %v", endpoint, err)
			} else {
				if log.GetLevel() >= log.DebugLevel {
					log.Debugf("Endpoint %v returned value: %v", endpoint, value)
				}
				if value.Type() == model.ValVector {
					if len(value.String()) > 0 {
						endpoint.ComparisonMetricValue = value.String()
						endpoint.Selected = true
						selected++
					}
				} else {
					log.Errorf("Endpoint %v returned unexpected type: %v", endpoint, value.Type())
				}
			}
		}
	}
	if selected > 0 {
		return nil
	}
	return fmt.Errorf("No valid/responding endpoints found in the provided list: %v", endpoints)
}
