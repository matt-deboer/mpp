package singlemostdata

import (
	"context"
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/matt-deboer/mpp/pkg/locator"
	"github.com/matt-deboer/mpp/pkg/selector"
	"github.com/prometheus/common/model"
)

func init() {
	s := &Selector{}
	selector.RegisterStrategy(s.Name(), s)
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
	return "local_storage_ingested_samples_total"
}

// Select chooses elligible prometheus endpoints out of the provided set
func (s *Selector) Select(endpoints []*locator.PrometheusEndpoint) (err error) {
	var mostDataIndex = -1
	var mostData float64
	for i, endpoint := range endpoints {
		endpoint.Selected = false
		if endpoint.QueryAPI != nil {
			value, err := endpoint.QueryAPI.Query(context.TODO(), "prometheus_local_storage_ingested_samples_total", time.Now())
			if err != nil {
				log.Errorf("Endpoint %v returned error: %v", endpoint, err)
			} else {
				if log.GetLevel() >= log.DebugLevel {
					log.Debugf("Endpoint %v returned value: %v", endpoint, value)
				}
				if value.Type() == model.ValVector {
					sampleValue := float64(value.(model.Vector)[0].Value)
					if sampleValue > mostData {
						mostData = sampleValue
						mostDataIndex = i
					}
				} else {
					log.Errorf("Endpoint %v returned unexpected type: %v", endpoint, value.Type())
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
