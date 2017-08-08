package kuberneteslocator

import (
	"context"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/ericchiang/k8s"
	"github.com/ghodss/yaml"
	"github.com/matt-deboer/mpp/pkg/locator"
	log "github.com/sirupsen/logrus"
)

type kubeLocator struct {
	labelSelector k8s.Option
	portName      string
	portNumber    int32
	namespace     string
	serviceName   string
	client        *k8s.Client
}

// NewKubernetesLocator generates a new marathon prometheus locator
func NewKubernetesLocator(kubeconfig, namespace, labelSelector, port, serviceName string) (locator.Locator, error) {

	var client *k8s.Client
	var err error
	if len(kubeconfig) > 0 {
		data, err := ioutil.ReadFile(kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("read kubeconfig: %v", err)
		}

		// Unmarshal YAML into a Kubernetes config object.
		var config k8s.Config
		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("unmarshal kubeconfig: %v", err)
		}

		log.Infof("Using kubeconfig %s", kubeconfig)
		client, err = k8s.NewClient(&config)
		if err != nil {
			return nil, err
		}
	} else {
		log.Infof("Using in-cluster kubeconfig")
		client, err = k8s.NewInClusterClient()
		if err != nil {
			return nil, err
		}
	}

	ls := new(k8s.LabelSelector)
	if len(labelSelector) > 0 {
		for _, part := range strings.Split(labelSelector, ",") {
			if strings.Contains(part, "=") {
				eq := strings.Split(part, "=")
				ls.Eq(eq[0], strings.Join(eq[1:], "="))
			}
		}
	}

	portNumber, _ := strconv.ParseInt(port, 10, 32)

	return &kubeLocator{
		client:        client,
		labelSelector: ls.Selector(),
		namespace:     namespace,
		portName:      port,
		portNumber:    int32(portNumber),
		serviceName:   serviceName,
	}, nil
}

// Endpoints provides a list of candidate prometheus endpoints
func (k *kubeLocator) Endpoints() ([]*locator.PrometheusEndpoint, error) {

	endpoints := []string{}
	if len(k.serviceName) > 0 {
		endp, err := k.client.CoreV1().GetEndpoints(context.Background(), k.serviceName, k.namespace)
		if err != nil {
			return nil, err
		}
		var port int32
		if len(endp.Subsets) > 0 {
			for _, p := range endp.Subsets[0].Ports {
				if p.GetProtocol() == "TCP" {
					if len(k.portName) > 0 {
						if k.portName == p.GetName() || p.GetPort() == k.portNumber {
							// 'port' flag was specified; match by name or port value
							port = p.GetPort()
							break
						}
					} else {
						// 'port' flag not specified; take the first (TCP) port we found
						port = p.GetPort()
						break
					}
				}
			}
			for _, a := range endp.Subsets[0].Addresses {
				endpoints = append(endpoints, fmt.Sprintf("http://%s:%d", a.GetIp(), port))
			}
		}
	} else {
		pods, err := k.client.CoreV1().ListPods(context.Background(), k.namespace, k.labelSelector)
		if err != nil {
			return nil, err
		}
		for _, pod := range pods.Items {
			var port int32
			for _, c := range pod.Spec.Containers {
				for _, p := range c.Ports {
					if p.GetProtocol() == "TCP" {
						if len(k.portName) > 0 {
							if k.portName == p.GetName() || p.GetContainerPort() == k.portNumber {
								// 'port' flag was specified; match by name or port value
								port = p.GetContainerPort()
							}
						} else {
							// 'port' flag not specified; take the first (TCP) port we found
							port = p.GetContainerPort()
						}
					}
					if port > 0 {
						break
					}
				}
				if port > 0 {
					break
				}
			}
			endpoints = append(endpoints, fmt.Sprintf("http://%s:%d", pod.Status.GetPodIP(), port))
		}
	}
	return locator.ToPrometheusClients(endpoints)
}
