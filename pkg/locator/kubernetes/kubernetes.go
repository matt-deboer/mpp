package kubernetes

import (
	"io"

	"github.com/matt-deboer/mpp/pkg/locator"
)

type kubeLocator struct {
	APIEndpoint  string
	AuthEndpoint string
}

// NewKubernetesLocator generates a new marathon prometheus locator
func NewKubernetesLocator(kubeconfig io.Reader) (locator.Locator, error) {
	return &kubeLocator{}, nil
}

// Endpoints provides a list of candidate prometheus endpoints
func (ml *kubeLocator) Endpoints() ([]*locator.PrometheusEndpoint, error) {
	return nil, nil
}
