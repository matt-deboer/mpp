package kuberneteslocator

import (
	"fmt"
	"strconv"

	"github.com/matt-deboer/mpp/pkg/locator"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type kubeLocator struct {
	labelSelector string
	portName      string
	portNumber    int32
	namespace     string
	serviceName   string
	clientset     *kubernetes.Clientset
}

// NewKubernetesLocator generates a new marathon prometheus locator
func NewKubernetesLocator(kubeconfig, namespace, labelSelector, port, serviceName string) (locator.Locator, error) {

	var config *rest.Config
	if len(kubeconfig) > 0 {
		cff, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
		config = cff
	} else {
		icc, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		config = icc
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	portNumber, _ := strconv.ParseInt(port, 10, 32)

	return &kubeLocator{
		clientset:     clientset,
		labelSelector: labelSelector,
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
		endp, err := k.clientset.Core().Endpoints(k.namespace).Get(k.serviceName)
		if err != nil {
			return nil, err
		}
		var port int32
		for _, p := range endp.Subsets[0].Ports {
			if p.Protocol == v1.ProtocolTCP {
				if len(k.portName) > 0 {
					if k.portName == p.Name || p.Port == k.portNumber {
						// 'port' flag was specified; match by name or port value
						port = p.Port
						break
					}
				} else {
					// 'port' flag not specified; take the first (TCP) port we found
					port = p.Port
					break
				}
			}
		}
		for _, a := range endp.Subsets[0].Addresses {
			endpoints = append(endpoints, fmt.Sprintf("http://%s:%d", a.IP, port))
		}
	} else {
		pods, err := k.clientset.Core().Pods(k.namespace).List(v1.ListOptions{
			LabelSelector: k.labelSelector,
		})
		if err != nil {
			return nil, err
		}
		for _, pod := range pods.Items {
			var port int32
			for _, c := range pod.Spec.Containers {
				for _, p := range c.Ports {
					if p.Protocol == v1.ProtocolTCP {
						if len(k.portName) > 0 {
							if k.portName == p.Name || p.ContainerPort == k.portNumber {
								// 'port' flag was specified; match by name or port value
								port = p.ContainerPort
							}
						} else {
							// 'port' flag not specified; take the first (TCP) port we found
							port = p.ContainerPort
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
			endpoints = append(endpoints, fmt.Sprintf("http://%s:%d", pod.Status.PodIP, port))
		}
	}
	return locator.ToPrometheusClients(endpoints)
}
