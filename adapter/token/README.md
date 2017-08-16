# Clear Harbor
This repository provides an authorization backend for Armada ingress (a.k.a.
 frontdoor) using Bluemix IAM service.

The name Clear Harbor follows Kubernetes use of maritime terminology, attempting
 to convey the idea that it protects the from the "rough seas" outside...

## Motivation

### Ingress Resources
An Ingress is a Kubernetes (k8s) resource that lets you configure an HTTP
 load balancer to expose Kubernetes services to clients outside the cluster.
 An Ingress resource supports:

 * Exposing services:
    * **Path based routing** (e.g., map the URL `/docs` to k8s service
      `documents`);
    * **Virtual hosting**, based on the HTTP `Host` header (e.g., mapping
      `Host: foo.example.com` to one group of services and `bar.example.com`
      to another).
 * Configuring SSL termination for each exposed host name.

See the [Ingress User Guide](http://kubernetes.io/docs/user-guide/ingress/)
 to learn more.

### Ingress Controllers

An Ingress controller is a k8s application that monitors Ingress resources
 via the Kubernetes API and updates the configuration of a load balancer
 in case of any changes. Different load balancers require different Ingress
 controller implementations. Typically, an Ingress controller is deployed
 as a pod in a cluster, along with the load balancer it controls.

See https://github.com/kubernetes/contrib/tree/master/ingress/controllers/
 to learn more about Ingress controllers.

### Authentication and Authorization

The Ingress Controller load balancer acts as the gateway to services running
 inside the k8s cluster. As such, Clear Harbor can provide seamless token
 validation for customer applications running on Armada Kubernetes clusters.
 Its goal is to make token validation, using Bluemix IAM service, easy
 to consume and ensure validation is correct, so that developers are not
 required to understand and implement the validation protocol themselves.

## User Workflow

### Deployment Architecture on Armada
Clear Harbor provides an authentication proxy that can be plugged into
 the (Frontdoor based) Armada Ingress Controller. Clear Harbor runs as
 one or more pods (e.g,. via a replication controller) on the customer's
 worker nodes and exposes a k8s `Service` object.
 See [rc.yaml](https://github.ibm.com/hrl-microservices/clearharbor/blob/master/build/rc.yaml)
 and [service.yaml](https://github.ibm.com/hrl-microservices/clearharbor/blob/master/build/service.yaml).

The only configuration required by Clear Harbor is the IAM service public
 key URL, which may be provided as an environment variable in the deployment
 definition.

> Please see [this](https://github.ibm.com/IAM/token-exchange/blob/master/validate_token.md)
>  for discussion of IAM keys and token validation.

### Defining Ingress Routes
Cluster administrators, wishing to expose services to the outside, can define
 two types of ingress routes: secured and unsecured.
 Access to unsecured routes will be forwarded to the target service directly,
 whereas all secure route access will be redirected to Clear Harbor for
 token validation.

Requests accessing secured routes are redirected to Clear Harbor for validation.
 Validation includes multiple steps (e.g., confirming tokens present, issued by
 Bluemix IAM service and within the correct time frame).
 Any validation step failure will return an HTTP failure code (i.e., 401) to the
 Ingress Controller and, through it, to the original client. In addition,
 Clear Harbor sets the HTTP `WWW-Authenticate` header to include the
 challenge, scope and error message. Failures may optionally include the
 validation failure description in the response body.
 If all validation steps pass, Clear Harbor will return an HTTP success
 response code (i.e., 200).

The repository provides a [sample](https://github.ibm.com/hrl-microservices/clearharbor/blob/master/build/ingress-with-auth.yaml)
 Ingress resource definition file for secured routes:

```yaml
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: ingress-auth
  annotations:
    nginx.org/location-snippets: |
        # this location requires AppID authentication using ClearHarbor
        auth_request /_auth-clearharbor;
    nginx.org/server-snippets: |
        location = /_auth-clearharbor {
                internal;
                proxy_pass_request_body     off;
                proxy_set_header            Content-Length "";
                proxy_set_header            Host $host;
                proxy_pass_request_headers  on;
                proxy_pass http://clearharbor.kube-system.svc.cluster.local/validate;
         }
spec:
  ...
```

The inclusion of both `nginx.org/location-snippets` and `nginx.org/server-snippets`
 annotations is required to enable secure access checks using Clear Harbor.
 Please see [Caveats](#Caveats) for a note regarding annontation use.
 All ingresses URLs matching rules defined in the `spec` will be subject to
 access checks.

If both secure and unsecure routes are needed, two separate Ingress resource
 definitions are required. Unsecured Ingress resources definition is
 unchancged, and defined as per the [Ingress User Guide](http://kubernetes.io/docs/user-guide/ingress/)).

### Token claims
Clear Harbor provides authorization by validating JWT tokens.
Such tokens can bear claims (https://tools.ietf.org/html/rfc7519#section-4).
Clear Harbor can extract claims from the request's tokens, and include them in
its response via HTTP headers of the form `X-Token-Claim-Name`, where `Name`
is a claim name.
Note that claim names are case sensitive and must match the claim name in the token
(see [choosing claims](#choosing-claims-to-be-included-in-the-response)).
The claim name will always be capitalized in the response header (i.e., `Name`, not `name`).
A header `X-Token-Claim-N` will contain the value of the claim named `N`.

#### Choosing claims to be included in the response
Token introspection is disabled by default. To enable a set of claims globally,
set the `TOKEN_CLAIMS` environment variable to include a comma separated list
of claims that will be returned (e.g `name1,name2,name3`).
Claim names are case-sensitive and must match the actual claim names in the token.
You can introspect hierarchical claims by using dot (`.`) as the hierarchy separator.
Dots will be converted to hypens (`-`) in response header names (i.e., calim `parent.child`
will be returned in header `X-Token-Claim-Parent-Child`).

Alternately, specify the claims on a per request basis by setting the
`X-Token-Claims` header in the Ingress proxy's request to Clear Harbor.
The value of this header should be in the same format as described for `TOKEN_CLAIMS`.
If both environment variable and header are specified, the header value has precedence.

If neither of the two options is used, the default behavior is to not include any
claims in the response.

```
$ export TOKEN=eyJraWQiOiIyMDE3MDQwMS0wMDowMDowMCIsIm..._oRCOA
$ curl -v -H "Authorization: Bearer $TOKEN" -H "X-Token-Claims: sub,iat,account" http://localhost:8080/validate
*   Trying 127.0.0.1...
* TCP_NODELAY set
* Connected to localhost (127.0.0.1) port 8080 (#0)
> GET /validate HTTP/1.1
> Host: localhost:8080
> User-Agent: curl/7.52.1
> Accept: */*
> Authorization: Bearer eyJraW...oRCOA
> X-Token-Claims: sub,iat,account
>
< HTTP/1.1 200 OK
< Request-Id: 0cab30a045e00d16
< X-Token-Claim-Account: map[bss:d51123d74ac88002189f7e6f45cf9e73]
< X-Token-Claim-Iat: 1499959533
< X-Token-Claim-Sub: ServiceId-71e9425f-3d3a-4665-a989-6769f9ebddaf
< Date: Sun, 16 Jul 2017 14:04:09 GMT
< Content-Length: 0
< Content-Type: text/plain; charset=utf-8
<
```

As can be seen in the example above, Clear Harbor supports returning arbitrary claim types.
The returned header value can be a string, a JSON numbers (which are
floating point by default), or arbitrary JSON objects (encoded as a Go encoded object string).

### Formatting Claims in a Single Header Value

Clear Harbor can also return claims in a formatted header value, based on a Go
[text/template](https://golang.org/pkg/text/template/) format.
The header name is `X-Token-Format-Claims` (and the corresponding environment variable
is `TOKEN_FORMAT_CLAIMS`. The same header name is used for both request and response.
Example:

```
$ curl -v -H "Authorization: Bearer $TOKEN" -H "X-Token-Format-Claims: {{.sub}}-{{.account.bss}}" http://localhost:8080/validate
*   Trying 127.0.0.1...
* TCP_NODELAY set
* Connected to localhost (127.0.0.1) port 8080 (#0)
> GET /validate HTTP/1.1
> Host: localhost:8080
> User-Agent: curl/7.52.1
> Accept: */*
> Authorization: Bearer eyJraW...oRCOA
> X-Token-Format-Claims: sub={{.sub}}; bss={{.account.bss}}
>
< HTTP/1.1 200 OK
< Request-Id: 24e1f53226618c09
< X-Token-Format-Claims: sub=ServiceId-71e9425f-3d3a-4665-a989-6769f9ebddaf; bss=d51123d74ac88002189f7e6f45cf9e73
< Date: Thu, 20 Jul 2017 13:24:15 GMT
< Content-Length: 0
< Content-Type: text/plain; charset=utf-8
<
```

## Developer Workflow

### Building

Clear Harbor is implemented in [Go](https://golang.org), so a working build
 environment is required (see [these instructions](https://golang.org/doc/install)).
 To simplify building code and related Docker images, the repository's
 root directory includes a Makefile.

```bash
$ make build docker.build
```

Go dependencies are managed in the `vendor` directory using [glide](https://github.com/Masterminds/glide)
Note that the Makefile is configured to use a private Docker registry, via
 the `DOCKER_REGISTRY_HOST` variable. This should be changed to the desired
 target registry, in which case the make command can be changed to push
 the newly built image (name using the `IMAGE_NAME` parameter):

```bash
$ make build docker.build docker.push
```

The Docker image uses [alpine](https://alpinelinux.org/) as its base,
 producing small images with no external dependencies (Go is configured to
 use static compilation).

### Directory and File Organization

All Go code is in the [top level directory](https://github.ibm.com/hrl-microservices/clearharbor/),
 including:

 * `config.go`: reading configuration values from command line and environment
 * `server.go`: HTTP server and handler (token extraction and validation)
 * `requestid.go`: extract or generate a unique/random request identifier for logging
 * `jwt_parser.go`: JWT parser
 * `publickey.go`: public key decoding (key itself is retrieved in `server.go`)

### Token Validation

The following steps are done in order to validate the token:

 1. Validate request contains an Authorization header
 1. Validate Authorization header contains a bearer token
 1. Validate token contains Auth token (it may also conain an ID Token)
 1. Validate the Auth token is signed by IAM (using its public key)
 1. Validate Auth token has not expired and is not used before it's valid

Failing to validate the token returns an HTTP error code the Ingress Controller.

### Redirecting Clients

Due to nginx auth-req module peculiarities, Clear Harbor may only respond with
 HTTP status codes `2xx` (to indicate success), `401` (to indicate authentication
 failure), and `403` (to indicate authorization failure). Currently, status code
 `403` is reserved for future use, and Clear Harbor will return 401 for all failures.
 Upon verification failure, Clear Harbor will include an HTTP header (named `X-AuthReq-Redirect`)
 indicating if (re-)authentication could be helpful before retrying access.
 The redirect target is application specific (e.g., a specific application URL or 
 an external identity provider) and should be configured via Ingress annotations.

The example nginx configuration below, shows how to redirect clients based
 on the response header from Clear Harbor. Note that the configuration is
 for illustration purpose only and is not necessarily a working configuration.

```apacheconf
location / { 
    auth_request /auth; # apply auth to this location
    # set variable from response header "X-AuthReq-Redirect"
    auth_request_set $auth_should_redirect $upstream_http_x_authreq_redirect;
    # set the redirect location from the ingress
    auth_request_set $auth_redirect_location "url-to-reauthenticate for this location"
    # continue processing 401 errors in a different location
    error_page 401 = @check_redirect;
    ...
}

location = /auth {
    proxy_pass "clearharbor localhost url";
    ...
}

location = @check_redirect {
    if ($auth_should_redirect) { # should redirect?
        return 302 $auth_redirect_location;
        # instead of using a variable for the location header value,  one could define
        # different @redirect locations based on target URL and select the appropriate one
        # via the error_page directive in the secured ingress location
    }

    return 401; # normal 401 processing
}
```

## Caveats and Future Extensions <a name="Caveats"></a>

 * The Frontdoor Ingress controller is based on the [nginxinc/nginx-ingress](https://github.com/nginxinc/kubernetes-ingress)
   image from Docker hub. The required annotation snippets used are available
   only in an (yet) unreleased version (current version is 0.7).
 * The customization mechanism is based on nginx specific [module](nginx.org/en/docs/http/ngx_http_auth_request_module.html)
   and configuration annotations. In the future, we expect to generalize this
   to intent based annotations, decoupled from nginx internal formats. That is,
   the annotation syntax will be proxy agnostic and the Ingress Controller will
   use proxy specific directives to configure the proxy.
 * Introspection of encrypted tokens via OAuth/OIDC is not supported. Clear Harbor validates clear text signed tokens
 * OAuth scopes are not used in token validation or request access authorization.
 * Clear Harbor retrieves the IAM public directly. If needed, public key
   retrieval may use the public certificate instead, allowing for expiration,
   CRL, etc. Without such mechanisms in place, Clear Harbor must fall back
   to heuristics (e.g., periodic refresh) in order to keep the public key
   updated.
