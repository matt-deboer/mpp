package locator

import (
	"fmt"
	"strings"

	"io/ioutil"
	"regexp"

	"github.com/prometheus/client_golang/api/prometheus"
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
	QueryAPI prometheus.QueryAPI
	Address  string
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
				endpoints = append(endpoints, &PrometheusEndpoint{QueryAPI: prometheus.NewQueryAPI(client), Address: addr})
			} else {
				errs = append(errs, err)
			}
		}
	}
	if len(errs) > 0 {
		return endpoints, fmt.Errorf("One or more errors occurred while creating clients: %#v", errs)
	}
	return endpoints, nil
}
