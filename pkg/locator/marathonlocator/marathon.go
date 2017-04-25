package marathonlocator

import (
	"fmt"

	log "github.com/Sirupsen/logrus"

	"github.com/matt-deboer/go-marathon"
	"github.com/matt-deboer/mpp/pkg/locator"
)

type marathonLocator struct {
	client        marathon.Marathon
	authEndpoint  string
	apps          []string
	authenticator *authenticator
}

func (ml *marathonLocator) String() string {
	return fmt.Sprintf("%T{url: %s, apps: %v}", ml, ml.client.GetMarathonURL(), ml.apps)
}

// NewMarathonLocator generates a new marathon prometheus locator
func NewMarathonLocator(marathonAPI string, prometheusApps []string, authEndpoint, principalSecret string, insecure bool) (locator.Locator, error) {

	ml := &marathonLocator{
		authEndpoint: authEndpoint,
		apps:         prometheusApps,
	}
	var client marathon.Marathon
	var err error
	config := marathon.NewDefaultConfig()
	config.URL = marathonAPI
	if len(principalSecret) > 0 {
		var authContext *authContext
		authContext, err = fromPrincipalSecret([]byte(principalSecret))
		if err != nil {
			return nil, fmt.Errorf("Failed to parse marathon princiapl secret: %v", err)
		}
		if len(authEndpoint) > 0 {
			authContext.AuthEndpoint = authEndpoint
		}
		authenticator := newAuthenticator(authContext, insecure)
		ml.authenticator = authenticator
		client, err = ml.authenticate(marathonAPI)
	} else {
		client, err = marathon.NewClient(config)
	}
	if err != nil {
		return nil, fmt.Errorf("Failed to construct marathon client: %v", err)
	}

	if ok, err := client.Ping(); !ok {
		return nil, err
	}
	ml.client = client
	return ml, nil
}

func (ml *marathonLocator) authenticate(marathonURL string) (marathon.Marathon, error) {
	config := marathon.NewDefaultConfig()
	config.URL = marathonURL
	token, err := ml.authenticator.authenticate()
	if err != nil {
		return nil, err
	}
	config.DCOSToken = token
	if log.GetLevel() >= log.DebugLevel {
		log.Debugf("Constructing marathon client with config: %#v", config)
	}
	return marathon.NewClient(config)
}

// Endpoints provides a list of candidate prometheus endpoints
func (ml *marathonLocator) Endpoints() ([]*locator.PrometheusEndpoint, error) {

	endpoints := []string{}
	for _, name := range ml.apps {
		app, err := ml.client.ApplicationBy(name, nil)
		if apiError, ok := err.(*marathon.APIError); ok && apiError.ErrCode == marathon.ErrCodeUnauthorized {
			client, err := ml.authenticate(ml.client.GetMarathonURL())
			if err != nil {
				return nil, err
			}
			ml.client = client
			app, err = ml.client.ApplicationBy(name, nil)
		}

		if err != nil {
			if apiError, ok := err.(*marathon.APIError); ok {
				log.Errorf("Failed to resolve marathon application '%s': %d, %v", name, apiError.ErrCode, err)
			} else {
				log.Errorf("Failed to resolve marathon application '%s': %v", name, err)
			}
		} else {
			for _, task := range app.Tasks {
				endpoints = append(endpoints, fmt.Sprintf("http://%s:%d", task.Host, task.Ports[0]))
			}
		}
	}
	return locator.ToPrometheusClients(endpoints)
}
