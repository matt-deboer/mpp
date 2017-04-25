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
	selector.RegisterStrategy("single-most-data", &Selector{})
}

// Selector implements selction of a single prometheus endpoint out of a provided set of endpoints
// which has the highest value of total ingested samples
type Selector struct {
}

// Select chooses elligible prometheus endpoints out of the provided set
func (s *Selector) Select(endpoints []*locator.PrometheusEndpoint) (targets []string, err error) {
	var mostDataIndex = -1
	var mostData float64
	for i, endpoint := range endpoints {
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
	if mostDataIndex >= 0 {
		return []string{endpoints[mostDataIndex].Address}, nil
	}
	return nil, fmt.Errorf("No valid/responding endpoints found in the provided list: %v", endpoints)
}
