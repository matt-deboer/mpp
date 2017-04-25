mpp
===

Multi-prometheus proxy (**mpp**) exists to forward incoming query requests to one of a set
of multiple prometheus instances deployed as HA duplicates of each other.


Motivation
---

As the recommended pattern for running Prometheus in HA mode is to [run multiple duplicate instances](https://github.com/prometheus/prometheus/issues/1500)
(same configuration, scraping the same targets independently), a method is needed to route queries
appropriately between those instances to provide a seemless experience for clients when individual
instance failures occur.

How it works
---

MPP acts as a proxy sitting in front of multiple prometheus instances, choosing one or more instances
to receive requests based on a configurable selector strategy. Candidate instances are found using
a configured _locator_ mechanism, of which [Marathon], [Kubernetes] and **static** are supported.

Discovery
---

Prometheus endpoints can be discovered via the Marathon API, the Kubernetes API, or by providing a
static list of endpoints.


Selection
---

Traffic is routed based on the chosen `--selector-strategy`:

- `single-most-data`: This strategy always routes traffic to a single prometheus endpoint, determined
  by whichever endpoint contains the _most_ data, measured by total metrics count.







