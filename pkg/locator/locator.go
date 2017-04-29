package locator

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"

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
	QueryAPI              prometheus.QueryAPI
	Error                 error
	Uptime                time.Duration
	Selected              bool
	Address               string
	ComparisonMetricValue interface{}
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
	for _, endpoint := range endpointURLs {
		addr := strings.Trim(endpoint, " ")
		if len(addr) > 0 {
			var uptime time.Duration
			var queryAPI prometheus.QueryAPI
			client, err := prometheus.New(prometheus.Config{
				Address: addr,
			})
			if err == nil {
				// Scape the /metrics endpoint of the individual prometheus instance, since
				// self-scaping of prometheus' own metrics might not be configured
				if log.GetLevel() >= log.DebugLevel {
					log.Debugf("Testing %s/metrics", addr)
				}
				scraped, err := ScrapeMetric(addr, "process_start_time_seconds")
				if err == nil && scraped != nil {
					processStartTimeSeconds := scraped.Value
					uptime = time.Duration(time.Now().UTC().Unix()-int64(processStartTimeSeconds)) * time.Second
					if log.GetLevel() >= log.DebugLevel {
						log.Debugf("Parsed current uptime for %s: %s", addr, uptime)
					}
					queryAPI = prometheus.NewQueryAPI(client)
					_, err = queryAPI.Query(context.TODO(), "up", time.Now())
					if err != nil && log.GetLevel() >= log.DebugLevel {
						log.Debugf("Query 'up' returned error: %v", err)
					}
				}
			}

			if err == nil {
				endpoints = append(endpoints, &PrometheusEndpoint{QueryAPI: queryAPI, Address: addr, Uptime: uptime})
			} else {
				log.Errorf("Failed to resolve build_info and uptime for %v: %v", addr, err)
				endpoints = append(endpoints, &PrometheusEndpoint{Address: addr, Uptime: time.Duration(0), Error: err})
			}
		}
	}
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("Unable to locate any potential endpoints")
	}
	return endpoints, nil
}

// LabeledValue represents a persed metric instance
type LabeledValue struct {
	Name   string
	Labels string
	Value  float64
}

func (lv *LabeledValue) String() string {
	return fmt.Sprintf("%s%s %f", lv.Name, lv.Labels, lv.Value)
}

// ScrapeMetric parses metrics in a simple fashion, returning
// the first instance of each metric for a given name; results may be unexpected
// for metrics with multiple instances
func ScrapeMetric(addr string, name string) (*LabeledValue, error) {

	resp, err := http.Get(fmt.Sprintf("%s/metrics", addr))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s/metrics returned %d", addr, resp.StatusCode)
	}

	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "#") {
			parts := strings.Split(line, " ")
			nameParts := strings.Split(parts[0], "{")
			if nameParts[0] == name {
				f := new(big.Float)
				_, err := fmt.Sscan(parts[1], f)
				if err == nil {
					v := &LabeledValue{Name: nameParts[0]}
					v.Value, _ = f.Float64()
					if len(nameParts) > 1 {
						v.Labels = "{" + nameParts[1]
					}
					return v, nil
				}
				return nil, fmt.Errorf("Failed to parse value for metric %s", line)
			}
		}
	}
	return nil, nil
}
