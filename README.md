![beaver logo](docs/bobr.png?raw=true)

# Wireguard Access

An intranet for exposing your kubernetes cluster resources internally.

Readme generated with [readme-generator-for-helm](https://github.com/bitnami/readme-generator-for-helm).

## Parameters

### Global parameters

| Name                   | Description                  | Value                |
| ---------------------- | ---------------------------- | -------------------- |
| `global.imageRegistry` | Global Docker image registry | `ghcr.io/kraudcloud` |

### Common parameters

| Name                                 | Description                                                                                                                      | Value          |
| ------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------- | -------------- |
| `endpoint.clientCIDR`                | CIDR range for client IPs. This is the range from which the wga pod will allocate IPs.                                           | `""`           |
| `endpoint.address`                   | Public address for the wireguard interface. Prefer using endpoint.service.loadBalancerIP                                         | `""`           |
| `endpoint.allowedIPs`                | List of IPs that are allowed to connect to from the wireguard interface                                                          | `""`           |
| `endpoint.logLevel`                  | Log level for the wireguard interface. error: 8, warn: 4, info: 0, debug: -4                                                     | `0`            |
| `endpoint.annotations`               | Additional annotations for the wireguard interface                                                                               | `{}`           |
| `endpoint.labels`                    | Additional labels for the wireguard interface                                                                                    | `{}`           |
| `endpoint.service.type`              | Kubernetes Service type.                                                                                                         | `LoadBalancer` |
| `endpoint.service.loadBalancerClass` | Kubernetes LoadBalancerClass to use                                                                                              | `""`           |
| `endpoint.service.loadBalancerIP`    | Kubernetes LoadBalancerIP to use                                                                                                 | `""`           |
| `endpoint.service.port`              | Kubernetes Service port                                                                                                          | `51820`        |
| `endpoint.service.annotations`       | Additional annotations for the Service                                                                                           | `{}`           |
| `endpoint.service.labels`            | Additional labels for the Service                                                                                                | `{}`           |
| `endpoint.image.name`                | endpoint image name                                                                                                              | `wga`          |
| `endpoint.image.tag`                 | endpoint image tag                                                                                                               | `""`           |
| `endpoint.image.pullPolicy`          | Image pull policy                                                                                                                | `""`           |
| `endpoint.resources.requests.memory` | Memory requests for the container                                                                                                | `128Mi`        |
| `endpoint.resources.limits.memory`   | Memory limits for the container                                                                                                  | `128Mi`        |
| `endpoint.resources.requests.cpu`    | CPU requests for the container                                                                                                   | `500m`         |
| `endpoint.resources.limits.cpu`      | CPU limits for the container                                                                                                     | `500m`         |
| `web.image.name`                     | Image name for the web component                                                                                                 | `wga-frontend` |
| `web.image.tag`                      | Image tag for the web component                                                                                                  | `main`         |
| `web.image.pullPolicy`               | Image pull policy for the web component                                                                                          | `""`           |
| `web.resources`                      | CPU/Memory resource requests/limits for the web component                                                                        | `{}`           |
| `web.authId`                         | Authentik ID for the web component                                                                                               | `""`           |
| `web.authSecret`                     | Authentik secret for the web component                                                                                           | `""`           |
| `web.authIssuer`                     | Authentik issuer endpoint                                                                                                        | `""`           |
| `web.authAuthorization`              | Authentik authorization endpoint                                                                                                 | `""`           |
| `web.debug`                          | Debug debug logging based on js-debug                                                                                            | `*`            |
| `web.service.type`                   | Kubernetes Service type for the web component                                                                                    | `ClusterIP`    |
| `web.annotations`                    | Additional annotations for the web component                                                                                     | `{}`           |
| `web.labels`                         | Additional labels for the web component                                                                                          | `{}`           |
| `ingress.enabled`                    | Enable ingress resource for the web component                                                                                    | `false`        |
| `ingress.annotations`                | Additional annotations for the Ingress resource                                                                                  | `{}`           |
| `ingress.hosts`                      | Ingress hosts for the web component                                                                                              | `[]`           |
| `ingress.tls`                        | Ingress TLS configuration                                                                                                        | `[]`           |
| `ingress.className`                  | Ingress class name for the web component                                                                                         | `""`           |
| `serviceAccount.create`              | Specifies whether a service account should be created. A service is required for the wga to communicate with the Kubernetes API. | `true`         |
| `serviceAccount.name`                | The name of the service account to use. If not set and create is true, a name is generated using the fullname template.          | `""`           |
| `clusterClient.enable`               | enable a daemonset to access other clusters wga via WireguardClusterClient CRD                                                   | `false`        |
