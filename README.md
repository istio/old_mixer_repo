<h1>The Istio Mixer</h1>
![Mixer](doc/logo.png)
<br/>
[![GoDoc](https://godoc.org/github.com/istio/mixer?status.svg)](http://godoc.org/github.com/istio/mixer)
[![Build Status](https://travis-ci.org/istio/mixer.svg?branch=master)](https://travis-ci.org/istio/mixer)
[![Go Report Card](https://goreportcard.com/badge/github.com/istio/mixer)](https://goreportcard.com/report/github.com/istio/mixer)
[![codecov.io](https://codecov.io/github/istio/mixer/coverage.svg?branch=master)](https://codecov.io/github/istio/mixer?branch=master)

The Istio mixer provides the foundation of the Istio service mesh
design. It is responsible for insulating the Istio proxy and
Istio-based services from details of the current execution
environment, as well as to implement the control policies
that Istio supports.

The Istio mixer provides three distinct features:

- *Precondition Checking*. The `Check` method enables
the caller to verify a number of preconditions before
responding to an incoming request from a service consumer.
Preconditions can include whether the service consumer
is on the service's whitelist, whether the service consumer
has the right access privilege, and more.

- *Telemetry Reporting*. The `Report` method enables services
to produce logging and monitoring streams intended for
service consumers.

- *Quota Management*. The `Quota` method enables services
to allocate and free quota on a number of dimensions, Quotas
are used as a relatively simple resource management tool to
provide some fairness between service consumers when contending
for limited service resources.

Please see the main Istio [README](https://raw.githubusercontent.com/istio/istio/master/README.md)
file to learn about the overall Istio project and how to get in touch with us. To learn how you can
contribute to any of the Istio components, including the mixer, please 
see the Istio [contribution guidelines](https://raw.githubusercontent.com/istio/istio/master/CONTRIBUTING.md).

The Istio mixer's [developer's guide](doc/dev/development.md) presents everything
you need to know to create, build, and test code for the mixer.
