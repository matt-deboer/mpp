package randomsticky

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
	selector.RegisterStrategy(s.Name(), s)
}

// Selector implements selction of a single prometheus endpoint out of a provided set of endpoints
// which has the highest value of total ingested samples
type Selector struct {
}

// Name provides the (unique) name of this strategy
func (s *Selector) Name() string {
	return "random-sticky"
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
			value, err := endpoint.QueryAPI.Query(context.TODO(), "prometheus_build_info", time.Now())
			if err != nil {
				log.Errorf("Endpoint %v returned error: %v", endpoint, err)
			} else {
				if log.GetLevel() >= log.DebugLevel {
					log.Debugf("Endpoint %v returned value: %v", endpoint, value)
				}
				if value.Type() == model.ValVector {
					endpoint.ComparisonMetricValue = value.String()
					endpoint.Selected = true
					selected++
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
