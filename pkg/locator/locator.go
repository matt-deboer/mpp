package locator

import (
	"context"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/prometheus/client_golang/api/prometheus"
	"github.com/prometheus/common/model"
)

var (
	splitter = regexp.MustCompile("\r?\n")
)

// Locator is a pluggable interface for locating prometheus endpoints
type Locator interface {

	// Endpoints provides a list of candidate prometheus endpoints
	Endpoints() ([]*PrometheusEndpoint, error)
}

// PrometheusEndpoint encapsulates a QueryAPI instance and its associated address
type PrometheusEndpoint struct {
	QueryAPI             prometheus.QueryAPI
	Uptime               time.Duration
	Selected             bool
	Address              string
	SelectionMetricValue interface{}
}

func (pe *PrometheusEndpoint) String() string {
	return pe.Address
}

type staticLocator struct {
	endpointsFile string
}

// NewEndpointsFileLocator returns a new Locator which reads
// a set of endpoints from a file path, one endpoint per line
func NewEndpointsFileLocator(endpointsFile string) Locator {
	return &staticLocator{endpointsFile: endpointsFile}
}

// Endpoints provides a list of candidate prometheus endpoints
func (sl *staticLocator) Endpoints() ([]*PrometheusEndpoint, error) {
	b, err := ioutil.ReadFile(sl.endpointsFile)
	if err != nil {
		return nil, err
	}
	return ToPrometheusClients(splitter.Split(strings.Trim(string(b), "\n"), -1))
}

// ToPrometheusClients generates prometheus Client objects from a provided list of URLs
func ToPrometheusClients(endpointURLs []string) ([]*PrometheusEndpoint, error) {
	endpoints := make([]*PrometheusEndpoint, 0, len(endpointURLs))
	errs := []error{}
	for _, endpoint := range endpointURLs {
		addr := strings.Trim(endpoint, " ")
		if len(addr) > 0 {
			client, err := prometheus.New(prometheus.Config{
				Address: addr,
			})
			if err == nil {
				queryAPI := prometheus.NewQueryAPI(client)
				result, err := queryAPI.Query(context.TODO(), "(time() - max(process_start_time_seconds{job=\"prometheus\"}))", time.Now())
				if log.GetLevel() >= log.DebugLevel {
					log.Debugf("Endpoint %v returned uptime result: %v", addr, result)
				}
				if err == nil {
					uptime := time.Duration(float64(result.(model.Vector)[0].Value)) * time.Second
					endpoints = append(endpoints, &PrometheusEndpoint{QueryAPI: prometheus.NewQueryAPI(client), Address: addr, Uptime: uptime})
					continue
				} else {
					errs = append(errs, err)
				}
			} else {
				errs = append(errs, err)
			}
			endpoints = append(endpoints, &PrometheusEndpoint{Address: addr, Uptime: time.Duration(0)})
		}
	}
	if len(errs) > 0 {
		return endpoints, fmt.Errorf("One or more errors occurred while creating clients: %#v", errs)
	}
	return endpoints, nil
}
