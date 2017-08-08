package router_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/matt-deboer/mpp/pkg/locator"
	"github.com/matt-deboer/mpp/pkg/router"
	_ "github.com/matt-deboer/mpp/pkg/selector/strategy/random"
)

const validUpResponse = `{
	"status":"success",
	"data":{
		"resultType":"vector",
		"result":[
			{
				"metric": {
					"__name__":"up",
					"instance":"1.2.3.4:9090",
					"job":"#NAME#"
				},
				"value":[1502134929.97,"1"]
			}
		]
	}
}`

const validMetricsResponse = `
# HELP prometheus_build_info A metric with a constant '1' value labeled by version, revision, branch, and goversion from which prometheus was built.
# TYPE prometheus_build_info gauge
prometheus_build_info{branch="master",goversion="go1.7.5",revision="bd1182d29f462c39544f94cc822830e1c64cf55b",version="1.5.2"} 1
# HELP process_start_time_seconds Start time of the process since unix epoch in seconds.
# TYPE process_start_time_seconds gauge
process_start_time_seconds 1.5021274556e+09
`

type mockLocator struct {
	endpoints []string
	mutex     sync.RWMutex
}

func (ml *mockLocator) UpdateEndpoints(endpoints []string) {
	ml.mutex.Lock()
	ml.endpoints = endpoints
	ml.mutex.Unlock()
}

func (ml *mockLocator) Endpoints() ([]*locator.PrometheusEndpoint, error) {
	ml.mutex.RLock()
	defer ml.mutex.RUnlock()
	return locator.ToPrometheusClients(ml.endpoints)
}

type mockPrometheus struct {
	available bool
	name      string
}

func (mp *mockPrometheus) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if mp.available {
		if r.URL.Path == "/api/v1/query" && r.URL.Query().Get("query") == "up" {
			w.Write([]byte(strings.Replace(validUpResponse, "#NAME#", mp.name, -1)))
		} else if r.URL.Path == "/metrics" {
			w.Write([]byte(validMetricsResponse))
		} else {
			w.Write([]byte(mp.name))
		}
	} else {
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}

}

func TestServeHTTPUnderLoadWithFailuresAndReselection(t *testing.T) {

	prom1 := &mockPrometheus{available: true, name: "prom1"}
	prom1Server := httptest.NewServer(prom1)
	defer prom1Server.Close()

	prom2 := &mockPrometheus{available: false, name: "prom2"}
	prom2Server := httptest.NewServer(prom2)
	defer prom2Server.Close()

	prom3 := &mockPrometheus{available: true, name: "prom3"}
	prom3Server := httptest.NewServer(prom3)
	defer prom3Server.Close()

	ml := &mockLocator{}
	ml.UpdateEndpoints([]string{prom1Server.URL, prom2Server.URL})

	ao1, err := router.ParseAffinityOption("sourceip")
	if err != nil {
		t.Fatal(err)
	}

	ao2, err := router.ParseAffinityOption("cookies")
	if err != nil {
		t.Fatal(err)
	}

	// router performs background selection 4 times per second
	r, err := router.NewRouter(250*time.Millisecond, []router.AffinityOption{*ao1, *ao2}, []locator.Locator{ml}, "random")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	mppServer := httptest.NewServer(r)
	defer mppServer.Close()

	requestsPerSecond := 256
	concurrency := 15
	testSeconds := 5

	results := makeRequests(t, mppServer.URL, requestsPerSecond, concurrency, testSeconds)
	// available endpoints are also swapped 4 times per second
	go swapEndpoints([]string{prom1Server.URL, prom2Server.URL, prom3Server.URL}, ml, testSeconds, 250*time.Millisecond)

	totalResults := 0
	countsByEndpoint := make(map[string]int)
	for result := range results {
		countsByEndpoint[result]++
		totalResults++
	}

	expectedResults := int(0.98 * float64(requestsPerSecond) * float64(testSeconds))
	assert.True(t, totalResults > expectedResults, "Expected at least %v results", expectedResults)
	assert.True(t, len(countsByEndpoint) == 2, "Expected 2 available endpoints to serve results")
}

func swapEndpoints(urls []string, ml *mockLocator, testSeconds int, frequency time.Duration) {
	totalURLs := len(urls)
	end := time.Now().Add(time.Duration(testSeconds) * time.Second)
	i := 0
	for time.Now().Before(end) {
		ml.UpdateEndpoints([]string{urls[i%totalURLs], urls[(i+1)%totalURLs]})
		time.Sleep(frequency)
		i++
	}
}

func makeRequests(t *testing.T, url string, maxQPS, concurrency, seconds int) chan string {
	results := make(chan string, maxQPS*concurrency)
	bucket := make(chan bool, maxQPS)
	go func() {
		end := time.Now().Add(time.Duration(seconds) * time.Second)
		for time.Now().Before(end) {
			start := time.Now()
			for i := 0; i < maxQPS; i++ {
				select {
				case bucket <- true:
				default:
				}
			}
			time.Sleep(time.Second - time.Now().Sub(start))
		}
		close(bucket)
	}()

	go func() {
		for i := 0; i < concurrency; i++ {
			go func(index int) {
				client := &http.Client{}
				for _ = range bucket {
					results <- makeRequest(t, client, url)
				}
				if index == 0 {
					close(results)
				}
			}(i)
		}
	}()
	return results
}

func makeRequest(t *testing.T, client *http.Client, url string) string {
	resp, err := client.Get(url)
	if err != nil {
		t.Error(err)
	} else {
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 OK; got %d", resp.StatusCode)
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Error(err)
		}
		return string(data)
	}
	return ""
}
