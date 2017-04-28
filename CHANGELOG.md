changelog
===

v0.2.0-a2
---

**fixes:**

- added `--kube-namespace` requirement for kubernetes deployments to fix bug in k8s locator

v0.2.0-a1
---

**features:**

- initial kubernetes support
- added metrics for
  - build info
  - affinity hits by type
  - selection events
  - requests by backend
  - repsonse time by backend
- added selector strategies:
  - `minimum-history`
  - `random`