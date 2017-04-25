package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/matt-deboer/mpp/pkg/locator"
	"github.com/matt-deboer/mpp/pkg/locator/marathonlocator"
	"github.com/matt-deboer/mpp/pkg/selector"
	_ "github.com/matt-deboer/mpp/pkg/selector/strategy/singlemostdata"
	"github.com/urfave/cli"
)

// Name is set at compile time based on the git repository
var Name string

// Version is set at compile time with the git version
var Version string

func main() {

	app := cli.NewApp()
	app.Name = Name
	app.Usage = `
		Launch a dynamically configured proxy over multiple prometheus endpoints
		which selects a single endpoint based on configurable criteria.
		`
	app.Version = Version
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name: "kubeconfig",
			Usage: `The path to a kubeconfig file used to communicate with the kubernetes api server
				to locate prometheus instances`,
			EnvVar: "MPP_KUBECONFIG",
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
			Name:   "insecure, k",
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
			EnvVar: "MPP_MARATHON_PRINCIPAL_SECRET",
		},
		cli.StringFlag{
			Name:   "static-endpoints",
			Usage:  `A comma-separated list of static prometheus endpoints to use`,
			EnvVar: "MPP_STATIC_ENDPOINTS",
		},
		cli.StringFlag{
			Name: "selector-strategy",
			Usage: `The selector type to use for choosing viable prometheus enpoint(s) from those located;
				valid choices include: 'single-most-data'`,
			Value:  "single-most-data",
			EnvVar: "MPP_SELECTOR_STRATEGY",
		},
		cli.IntFlag{
			Name: "selection-interval",
			Usage: `The interval (in seconds) at which selections are performed; note that selection is
				automatically performed upon backend failures`,
			Value: 15,
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
		strategy := c.String("selector-strategy")
		interval := time.Duration(c.Int("selection-interval")) * time.Second
		locators := parseLocators(c, app)

		s, err := selector.NewSelector(strategy, locators)
		if err != nil {
			log.Fatal(err)
		}

		handler := newProxyHandler(s)
		server := &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: handler,
		}

		go func() {
			for {
				if log.GetLevel() >= log.DebugLevel {
					log.Debugf("Handler selection is sleeping for %s", interval)
				}
				time.Sleep(interval)
				handler.doSelection()
			}
		}()

		log.Infof("mpp is listening on port %d", port)
		server.ListenAndServe()
	}
	app.Run(os.Args)

}

func parseDuration(c *cli.Context, flag string) time.Duration {
	stringValue := c.String(flag)
	duration, err := time.ParseDuration(stringValue)
	if err != nil {
		log.Fatalf("Invalid duration for '%s': '%s': %v", flag, stringValue, err)
	}
	return duration
}

func parseLocators(c *cli.Context, app *cli.App) []locator.Locator {
	var locators []locator.Locator

	insecure := c.Bool("insecure")
	staticEndpoints := c.String("static-endpoints")
	kubeconfig := c.String("kubeconfig")
	marathonURL := c.String("marathon-url")

	if len(staticEndpoints) > 0 {
		locators = append(locators, locator.NewStaticLocator(strings.Split(staticEndpoints, ",")))
	}

	if len(kubeconfig) > 0 {

	}

	if len(marathonURL) > 0 {
		marathonAppsString := c.String("marathon-apps")
		if len(marathonAppsString) == 0 {
			argError(c, "'marathon-apps' is required when 'marathon-url' is specified")
		}
		marathonApps := strings.Split(marathonAppsString, ",")
		marathonPrincipalSecret := c.String("marathon-principal-secret")
		marathonAuthEndpoint := c.String("marathon-auth-endpoint")
		locator, err := marathonlocator.NewMarathonLocator(marathonURL, marathonApps, marathonAuthEndpoint, marathonPrincipalSecret, insecure)
		if err != nil {
			log.Fatalf("Failed to create marathon locator: %v", err)
		}
		locators = append(locators, locator)
	}
	if len(locators) == 0 {
		argError(c, "At least one locator mechanism must be configured; specify at least one of: --marathon-url, --kubeconfig, --static-endpoints")
	}

	return locators
}

func argError(c *cli.Context, msg string, args ...interface{}) {
	log.Errorf(msg+"\n", args...)
	cli.ShowAppHelp(c)
	os.Exit(1)
}
