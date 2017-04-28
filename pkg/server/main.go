package main // import "github.com/matt-deboer/mpp/pkg/server"

import (
	"fmt"
	"os"
	"strings"
	"time"

	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/matt-deboer/mpp/pkg/locator"
	"github.com/matt-deboer/mpp/pkg/locator/kuberneteslocator"
	"github.com/matt-deboer/mpp/pkg/locator/marathonlocator"
	"github.com/matt-deboer/mpp/pkg/router"
	_ "github.com/matt-deboer/mpp/pkg/selector/strategy/minimumhistory"
	_ "github.com/matt-deboer/mpp/pkg/selector/strategy/random"
	_ "github.com/matt-deboer/mpp/pkg/selector/strategy/singlemostdata"
	"github.com/matt-deboer/mpp/pkg/version"
	"github.com/urfave/cli"
)

func main() {

	app := cli.NewApp()
	app.Name = version.Name
	app.Version = version.Version
	app.Usage = `Launch a dynamically configured proxy over multiple prometheus endpoints
		which selects endpoints based on configurable criteria.
		`
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name: "kubeconfig",
			Usage: `The path to a kubeconfig file used to communicate with the kubernetes api server
				to locate prometheus instances`,
			EnvVar: "MPP_KUBECONFIG",
		},
		cli.StringFlag{
			Name:   "kube-service-name",
			Usage:  `The service name used to locate prometheus endpoints; take precedence over 'kube-pod-label-selector'`,
			EnvVar: "MPP_KUBE_SERVICE_NAME",
		},
		cli.StringFlag{
			Name:   "kube-pod-label-selector",
			Usage:  `The label selector used to find prometheus pods`,
			EnvVar: "MPP_KUBE_POD_LABEL_SELECTOR",
		},
		cli.StringFlag{
			Name:   "kube-namespace",
			Usage:  `The namespace in which prometheus pods/endpoints exist`,
			EnvVar: "MPP_KUBE_NAMESPACE",
		},
		cli.StringFlag{
			Name:   "kube-port",
			Usage:  `The port (name or number) where prometheus is listening on individual pods/endpoints`,
			EnvVar: "MPP_KUBE_PORT",
		},
		cli.StringFlag{
			Name:   "marathon-url",
			Usage:  `The URL for the marathon API endpoint used to locate prometheus instances`,
			EnvVar: "MPP_MARATHON_URL",
		},
		cli.StringFlag{
			Name: "marathon-apps",
			Usage: `A comma-separated list of marathon app IDs whose tasks will be queried for
				prometheus endpoints`,
			EnvVar: "MPP_MARATHON_APPS",
		},
		cli.StringFlag{
			Name:   "insecure-certs, k",
			Usage:  `Whether connections to https endpoints with unverifiable certs are allowed`,
			EnvVar: "MPP_INSECURE_CERTS",
		},
		cli.StringFlag{
			Name:   "marathon-principal-secret",
			Usage:  `The principal secret used to handle authentication with marathon `,
			EnvVar: "MPP_MARATHON_PRINCIPAL_SECRET",
		},
		cli.StringFlag{
			Name: "marathon-auth-endpoint",
			Usage: `The authentication endpoint to use with the 'marathon-principal-secret', overriding the value
				contained within the secret`,
			EnvVar: "MPP_MARATHON_AUTH_ENDPOINT",
		},
		cli.StringFlag{
			Name: "endpoints-file",
			Usage: `A file path containing a list of endpoints to use, one per line. This file is re-read at every
				selection interval`,
			EnvVar: "MPP_ENDPOINTS_FILE",
		},
		cli.StringFlag{
			Name: "routing-strategy",
			Usage: `The strategy to use for choosing viable prometheus enpoint(s) from those located;
				valid choices include: 'single-most-data', 'random', 'minimum-history'`,
			Value:  "single-most-data",
			EnvVar: "MPP_ROUTING_STRATEGY",
		},
		cli.StringFlag{
			Name: "selection-interval",
			Usage: `The interval at which selections are performed; note that selection is
				automatically performed upon backend failures`,
			Value:  "10s",
			EnvVar: "MPP_SELECTION_INTERVAL",
		},
		cli.StringFlag{
			Name: "affinity-options",
			Usage: `A comma-separated list of sticky-session modes to enable, of which 'cookie', and 'ip' 
				are valid options`,
			Value:  "cookies",
			EnvVar: "MPP_AFFINITY_OPTIONS",
		},
		cli.IntFlag{
			Name:   "port",
			Value:  9090,
			Usage:  "The port on which the proxy will listen",
			EnvVar: "MPP_PORT",
		},
		cli.BoolFlag{
			Name:   "verbose, V",
			Usage:  "Log debugging information",
			EnvVar: "MPP_VERBOSE",
		},
	}
	app.Action = func(c *cli.Context) {

		if c.Bool("verbose") {
			log.SetLevel(log.DebugLevel)
		}

		port := c.Int("port")
		strategy := c.String("routing-strategy")
		interval := parseDuration(c, "selection-interval")
		locators := parseLocators(c)
		affinityOptions := parseAffinityOptions(c)

		router, err := router.NewRouter(interval, affinityOptions,
			locators, strings.Split(strategy, ":")...)
		if err != nil {
			log.Fatal(err)
		}

		server := &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: newMPPHandler(router),
		}
		log.Infof("mpp is listening on port %d", port)
		server.ListenAndServe()
	}
	app.Run(os.Args)

}

func parseDuration(c *cli.Context, flag string) time.Duration {
	stringValue := c.String(flag)
	duration, err := time.ParseDuration(stringValue)
	if err != nil {
		argError(c, "Invalid duration for '%s': '%s': %v", flag, stringValue, err)
	}
	return duration
}

func parseAffinityOptions(c *cli.Context) []router.AffinityOption {
	opts := make([]router.AffinityOption, 0, 2)
	for _, o := range strings.Split(strings.Trim(c.String("affinity-options"), " \n"), ",") {
		opt, err := router.ParseAffinityOption(o)
		if err != nil {
			argError(c, "Invalid value for affinity-options '%s'", o)
		} else {
			opts = append(opts, *opt)
		}
	}
	return opts
}

func parseLocators(c *cli.Context) []locator.Locator {
	var locators []locator.Locator

	insecure := c.Bool("insecure-certs")
	endpointsFile := c.String("endpoints-file")
	kubeconfig := c.String("kubeconfig")
	kubeNamespace := c.String("kube-namespace")
	kubeServiceName := c.String("kube-service-name")
	kubePodLabelSelector := c.String("kube-pod-label-selector")
	marathonURL := c.String("marathon-url")

	if len(endpointsFile) > 0 {
		locators = append(locators, locator.NewEndpointsFileLocator(endpointsFile))
	}

	if len(kubeNamespace) > 0 {
		if len(kubeServiceName) == 0 && len(kubePodLabelSelector) == 0 {
			argError(c, "Kubernetes locator requires one of either 'kube-service-name' or 'kube-pod-label-selector'")
		}
		kubePort := c.String("kube-port")
		locator, err := kuberneteslocator.NewKubernetesLocator(kubeconfig,
			kubeNamespace, kubePodLabelSelector, kubePort, kubeServiceName)
		if err != nil {
			log.Fatalf("Failed to create kubernetes locator: %v", err)
		}
		locators = append(locators, locator)
	}

	if len(marathonURL) > 0 {
		marathonAppsString := c.String("marathon-apps")
		if len(marathonAppsString) == 0 {
			argError(c, "'marathon-apps' is required when 'marathon-url' is specified")
		}
		marathonApps := strings.Split(marathonAppsString, ",")
		marathonPrincipalSecret := c.String("marathon-principal-secret")
		marathonAuthEndpoint := c.String("marathon-auth-endpoint")
		locator, err := marathonlocator.NewMarathonLocator(marathonURL,
			marathonApps, marathonAuthEndpoint, marathonPrincipalSecret, insecure)
		if err != nil {
			log.Fatalf("Failed to create marathon locator: %v", err)
		}
		locators = append(locators, locator)
	}
	if len(locators) == 0 {
		argError(c, `At least one locator mechanism must be configured; specify at least one of: `+
			`--marathon-url, --kubeconfig/--kube-namespace/--kube-service-name/--kube-pod-label-selector, --static-endpoints`)
	}

	return locators
}

func argError(c *cli.Context, msg string, args ...interface{}) {
	log.Errorf(msg+"\n", args...)
	cli.ShowAppHelp(c)
	os.Exit(1)
}
