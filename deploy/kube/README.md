# Istio Mixer K8s Configuration

Intended for our demo, these configurations stand up an instance of the mixer, a Prometheus instance that scapes metrics
the mixer exposes, and a Grafana instance to render those metrics. The easiest way to stand up these deployments is to
run (from the Istio mixer root directory):

    $ kubectl create configmap testdata-config --from-file=testdata/globalconfig.yml --from-file=testdata/serviceconfig.yml --from-file=testdata/prometheus.yaml
    $ kubectl apply -f ./deploy/kube/
    
If you're using something like [minikube](https://github.com/kubernetes/minikube) to test locally, you can call the
mixer server with our test client (`mixc`), providing some attribute values:

    $ MIXS=$(minikube service mixer --url --format "{{.IP}}:{{.Port}}")
    $ bazel run //cmd/client:mixc -- check -m $MIXS -a source.name=source,target.name=target,api.name=myapi,response.code=200,response.latency=100,client.id=$USER

## mixer.yaml
`mixer.yaml` describes a deployment and service for the mixer server binary. The deployment annotates the pods it
creates with a `prometheus_port` annotation which is required by our Prometheus deployment (see [prometheus.yaml](#prometheus)
below as well as the prometheus configuration in `//testdata/prometheus.yaml`).

Two pieces of configuration are required to run the mixer: a global configuration and a service configuration. Example
configurations can be found in the `//testdata` directory (`//` indicates the root of the Istio Mixer project directory).
These configurations are expected to be mounted in two files at `/etc/opt/mixer/`. We use a configmap named `testdata-config`
to provide these configurations. Usually this configmap is created from the `//testdata/` directory by:

<a name="configmap_command"></a>

    $ kubectl create configmap testdata-config --from-file=testdata/globalconfig.yml --from-file=testdata/serviceconfig.yml --from-file=testdata/prometheus.yaml

A file can be created to capture this configmap by running:

    $ kubectl get configmap testdata-config -o yaml > ./testdata-config.yaml

We do not check in this file since we consider the yaml files in `//testdata` the source of truth for this config.

## grafana.yaml
`grafana.yaml` describes a deployment and service for Grafana, based on the [`grafana/grafana` docker image](https://hub.docker.com/r/grafana/grafana/).
The Grafana UI is exposed on port `3000`. This configuration stores Grafana's data in the `/data/grafana` dir, which we
configure to be an ephemeral directory. To persist your Grafana configuration for longer than the lifetime of the `grafana`
pod, map this directory to a real volume.

If you're using minikube to test these deployments locally, get to the UI by running:

    $ minikube service grafana
    
We also need to add our Prometheus installation data source, which is at `http://prometheus:9090/` (no auth required,
access via proxy).


## <a name="prometheus"></a> prometheus.yaml
`prometheus.yaml` describes a deployment and service for a Prometheus collection server. Prometheus expects its config to
be mounted at `/etc/opt/mixer/prometheus.yaml`. An example configuration that works with these deployments is stored in
`//testdata/`. For simplicity we shove this into the same configmap the mixer serves from, and the [configmap command above](#configmap_command)
includes the Prometheus configuration. 

When the mixer is configured with the `prometheus` adapter it hosts metrics on an HTTP server at port `42422` (this can
be changed in the Prometheus adapter configuration). The `mixer.yaml` deployment annotates all mixer pods with this port
number, and the Prometheus configuration at `//testdata/prometheus.yaml` uses it to compute the URL to scrape for metrics.

## localregistry.yaml
`localregistry.yaml` is a copy of Kubernete's local registry configuration, and is included to make it easier to test
the mixer server. Run the registry (`kubectl apply -f ./deploy/kube/localregistry.yaml`), and update `mixer.yaml` to use
the image `localhost:5000/mixer`. After the registry server is running, expose it locally by executing:

    $ kubectl port-forward --namespace kube-system $POD 5000:5000

If you're testing locally with minikube, `$POD` can be set with:

    $ POD=$(kubectl get pods --namespace kube-system -l k8s-app=kube-registry \
      -o template --template '{{range .items}}{{.metadata.name}} {{.status.phase}}{{"\n"}}{{end}}' \
      | grep Running | head -1 | cut -f1 -d' ')

Then you can build the mixer, create a docker image, and push it to the local registry, by running:

    $ bazel build ...:all
    $ bazel run //docker:mixer localhost:5000/mixer
    $ docker push localhost:5000/mixer
