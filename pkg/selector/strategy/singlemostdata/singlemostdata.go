package singlemostdata

import (
	"fmt"
	"net/url"

	log "github.com/Sirupsen/logrus"
	"github.com/matt-deboer/mpp/pkg/locator"
	"github.com/matt-deboer/mpp/pkg/selector"
)

const (
	comparisonMetricName = "prometheus_local_storage_ingested_samples_total"
)

func init() {
	s := &Selector{}
	selector.RegisterStrategy(s.Name(), func(args ...string) (selector.Strategy, error) {
		return s, nil
	})
}

// Selector implements selction of a single prometheus endpoint out of a provided set of endpoints
// which has the highest value of total ingested samples
type Selector struct {
}

// Name provides the (unique) name of this strategy
func (s *Selector) Name() string {
	return "single-most-data"
}

// Description provides a human-readable description for this strategy
func (s *Selector) Description() string {
	return "Selects the single prometheus instance with the most ingested samples"
}

// ComparisonMetricName gets the name of the comparison metric/calculation used to make a selection
func (s *Selector) ComparisonMetricName() string {
	return comparisonMetricName
}

// RequiresStickySessions answers whether this strategy needs sticky sessions
func (s *Selector) RequiresStickySessions() bool {
	return false
}

// NextIndex returns the index of the target that should be used to field the next request
func (s *Selector) NextIndex(targets []*url.URL) int {
	return 0
}

// Select chooses elligible prometheus endpoints out of the provided set
func (s *Selector) Select(endpoints []*locator.PrometheusEndpoint) (err error) {
	var mostDataIndex = -1
	var mostData int64
	for i, endpoint := range endpoints {
		endpoint.Selected = false
		if endpoint.QueryAPI != nil {
			scraped, err := locator.ScrapeMetric(endpoint.Address, "prometheus_local_storage_ingested_samples_total")
			if err != nil {
				log.Errorf("Endpoint %v returned error: %v", endpoint, err)
				endpoint.Error = err
			} else {
				if log.GetLevel() >= log.DebugLevel {
					log.Debugf("Endpoint %v returned value: %v", endpoint, scraped)
				}
				sampleValue := int64(scraped.Value)
				endpoint.ComparisonMetricValue = sampleValue
				if sampleValue > mostData {
					mostData = sampleValue
					mostDataIndex = i
				}
			}
		}
	}
	if mostDataIndex >= 0 {
		endpoints[mostDataIndex].Selected = true
		return nil
	}
	return fmt.Errorf("No valid/responding endpoints found in the provided list: %v", endpoints)
}
