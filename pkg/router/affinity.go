package router

import (
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/url"

	log "github.com/sirupsen/logrus"
	"github.com/hashicorp/golang-lru"
)

// AffinityOption represents supported options for session affinity
type AffinityOption uint8

const cookieName = "MPP.Route"

const (
	// AffinityByCookies implies session affinity by cookies
	AffinityByCookies AffinityOption = iota
	// AffinityBySourceIP implies session affinity by the source ip
	AffinityBySourceIP
)

var (
	ipRoutes, _ = lru.New(256)
)

var affinityOptionStrings = []string{"cookies", "sourceip"}

// ParseAffinityOption returns a AffinityOption for a provided string representation
func ParseAffinityOption(value string) (*AffinityOption, error) {
	for i, opt := range affinityOptionStrings {
		if value == opt {
			o := AffinityOption(i)
			return &o, nil
		}
	}
	return nil, fmt.Errorf("'%s' is not avalid AffinityOption", value)
}

func (o AffinityOption) String() string {
	return affinityOptionStrings[int(o)]
}

func ipv4toInt(ipv4Address net.IP) int64 {
	ipv4Int := big.NewInt(0)
	ipv4Int.SetBytes(ipv4Address.To4())
	return ipv4Int.Int64()
}

func newAffinityProvider(options []AffinityOption) *affinityProvider {
	a := &affinityProvider{}
	for _, opt := range options {
		if opt == AffinityByCookies {
			if log.GetLevel() >= log.DebugLevel {
				log.Debugf("Affinity by %v enabled", AffinityByCookies)
			}
			a.cookiesEnabled = true
		} else if opt == AffinityBySourceIP {
			if log.GetLevel() >= log.DebugLevel {
				log.Debugf("Affinity by %v enabled", AffinityBySourceIP)
			}
			a.sourceIPEnabled = true
		}
	}
	return a
}

// affinityProvider supports session affinity for backend routing
type affinityProvider struct {
	cookiesEnabled  bool
	sourceIPEnabled bool
}

// locate the preferred target (if any), based on selected affinity option(s)
func (a *affinityProvider) preferredTarget(req *http.Request, router *Router) *url.URL {
	if len(router.selection.Selection) > 1 {
		if a.cookiesEnabled {
			cookie, err := req.Cookie(cookieName)
			if cookie != nil {
				if log.GetLevel() >= log.DebugLevel {
					log.Warnf("Found cookie %v", cookie)
				}
				u, err := url.Parse(cookie.Value)
				if err != nil {
					log.Errorf("Sticky cookie contained unparsable url %s: %v", cookie.Value, err)
				} else if !contains(router.selection.Selection, u) {
					if log.GetLevel() >= log.DebugLevel {
						log.Debugf("Sticky cookie target %v is no longer valid", u)
					}
				} else {
					router.metrics.affinityHits.WithLabelValues(AffinityByCookies.String()).Inc()
					return u
				}
			} else {
				if log.GetLevel() >= log.DebugLevel {
					log.Warnf("Cookie %v not found: %v", cookieName, err)
				}
			}
		}
		if a.sourceIPEnabled {
			if u, ok := ipRoutes.Get(getSourceIPKey(req)); ok {
				router.metrics.affinityHits.WithLabelValues(AffinityBySourceIP.String()).Inc()
				return u.(*url.URL)
			}
		}
	}
	return nil
}

// returns the source IP in int64 format
func getSourceIPKey(req *http.Request) int64 {
	sourceIP := req.Header.Get("X-Forwarded-For")
	if len(sourceIP) == 0 {
		sourceIP = req.RemoteAddr
	}
	return ipv4toInt(net.ParseIP(sourceIP))
}

// cache the preferred target, based on selected affinity option(s)
func (a *affinityProvider) savePreferredTarget(w http.ResponseWriter, req *http.Request, needsCookie bool) {
	if needsCookie {
		backend := backend(req.URL)
		http.SetCookie(w, &http.Cookie{
			Name:     cookieName,
			Value:    backend,
			HttpOnly: true,
		})
		if log.GetLevel() >= log.DebugLevel {
			log.Warnf("Setting cookie %s for backend %v", cookieName, backend)
		}
	}
	if a.sourceIPEnabled {
		ipRoutes.Add(getSourceIPKey(req), req.URL)
	}
}
