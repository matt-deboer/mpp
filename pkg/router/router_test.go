package router

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/jsonq"

	"github.com/matt-deboer/mpp/pkg/locator"
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
					"job":"#NAME#",
				},
				"value":[1502134929.97,"1"]
			}
		]
	}
}`

// We need multiple mock backends, some of which
// are not viable

// We also need multiple clients to be constantly
// hitting mpp, causing re-selections to occur

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
	// if body, ok := metrics[r.URL.Path]; ok {
	// 	w.Write([]byte(body))
	// } else {
	// 	http.NotFound(w, r)
	// }

	if r.URL.Path == "/api/v1/query" && r.URL.Query().Get("query") == "up" {
		if mp.available {
			w.Write([]byte(strings.Replace(validUpResponse, "#NAME#", mp.name, -1)))
		} else {
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
		}
	}
}

func TestServeHTTPUnderLoadWithFailuresAndReselection(t *testing.T) {

	prom1 := &mockPrometheus{available: true, name: "prom1"}
	prom1Server := httptest.NewServer(prom1)
	defer prom1Server.Close()

	prom2 := &mockPrometheus{available: false, name: "prom2"}
	prom2Server := httptest.NewServer(prom2)
	defer prom2Server.Close()

	locators := []locator.Locator{&mockLocator{endpoints: []string{prom1Server.URL, prom2Server.URL}}}
	r, err := NewRouter(time.Second, []AffinityOption{}, locators, "random")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	mppServer := httptest.NewServer(r)
	defer mppServer.Close()
	// do tests....

	resp, err := http.Get(fmt.Sprintf("%s/api/v1/query?query=up", mppServer.URL))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200; got %d", resp.StatusCode)
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	jsonData := make(map[string]interface{})
	json.Unmarshal(data, &jsonData)

	jsonData["data"]
	jq := jsonq.NewQuery(data)
	// If we make a request now, we should get an answer from prom1, because prom2 is not availble

	// We update the endpoints list to remove prom1 and add prom3 -- if we make a new request, we should
	// get result from prom3 now

}
