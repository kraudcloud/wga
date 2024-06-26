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
| `podAnnotations`                     | Additional annotations for wga pods                                                                                              | `{}`           |
| `nodeSelector`                       | Node labels for pod assignment                                                                                                   | `{}`           |
| `tolerations`                        | List of node taints to tolerate (requires Kubernetes >=1.6)                                                                      | `[]`           |
| `affinity`                           | A dictionary containing affinity settings for wga pods                                                                           | `{}`           |
| `endpoint.clientCIDR`                | CIDR range for client IPs. This is the range from which the wga pod will allocate IPs.                                           | `""`           |
| `endpoint.address`                   | Public address for the wireguard interface. Prefer using endpoint.service.loadBalancerIP                                         | `""`           |
| `endpoint.allowedIPs`                | List of IPs that are allowed to connect to from the wireguard interface                                                          | `""`           |
| `endpoint.logLevel`                  | Log level for the wireguard interface. error: 8, warn: 4, info: 0, debug: -4                                                     | `0`            |
| `endpoint.annotations`               | Additional annotations for the wireguard interface                                                                               | `{}`           |
| `endpoint.labels`                    | Additional labels for the wireguard interface                                                                                    | `{}`           |
| `endpoint.resources`                 | CPU/Memory resource requests/limits for the wgap pod.                                                                            | `{}`           |
| `endpoint.privateKeySecretName`      | secret name for the private key of the wireguard interface. Should contain a single `privateKey` entry                           | `""`           |
| `endpoint.service.type`              | Kubernetes Service type.                                                                                                         | `LoadBalancer` |
| `endpoint.service.loadBalancerClass` | Kubernetes LoadBalancerClass to use                                                                                              | `""`           |
| `endpoint.service.loadBalancerIP`    | Kubernetes LoadBalancerIP to use                                                                                                 | `""`           |
| `endpoint.service.port`              | Kubernetes Service port                                                                                                          | `51820`        |
| `endpoint.service.annotations`       | Additional annotations for the Service                                                                                           | `{}`           |
| `endpoint.service.labels`            | Additional labels for the Service                                                                                                | `{}`           |
| `endpoint.image.name`                | endpoint image name                                                                                                              | `wga`          |
| `endpoint.image.tag`                 | endpoint image tag                                                                                                               | `""`           |
| `endpoint.image.pullPolicy`          | Image pull policy                                                                                                                | `""`           |
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

### Cluster client

| Name                      | Description                                                                    | Value   |
| ------------------------- | ------------------------------------------------------------------------------ | ------- |
| `clusterClient.enable`    | enable a daemonset to access other clusters wga via WireguardClusterClient CRD | `false` |
| `clusterClient.resources` | CPU/Memory resource requests/limits for the clusterClient component            | `{}`    |

### dns

| Name                     | Description                                                                        | Value                |
| ------------------------ | ---------------------------------------------------------------------------------- | -------------------- |
| `dns.enabled`            | whether DNS is enabled for wga.                                                    | `false`              |
| `dns.image.tag`          | Image tag for the dns component                                                    | `2.84`               |
| `dns.image.repository`   | Image repository for the dns component                                             | `tschaffter/dnsmasq` |
| `dns.image.pullPolicy`   | Image pull policy for the dns component                                            | `IfNotPresent`       |
| `dns.port`               | DNS port for the dnsmasq service                                                   | `53`                 |
| `dns.serviceType`        | Kubernetes Service type.                                                           | `LoadBalancer`       |
| `dns.serviceAnnotations` | Additional annotations for the Service                                             | `{}`                 |
| `dns.loadBalancerClass`  | Kubernetes LoadBalancerClass to use.                                               | `""`                 |
| `dns.replicaCount`       | Number of replicas for the dns component                                           | `1`                  |
| `dns.address`            | Public IP address for the dnsmasq service. Prefer using dns.service.loadBalancerIP | `""`                 |
| `dns.resources`          | CPU/Memory resource requests/limits for the dns component                          | `{}`                 |
| `dns.noDefaultConfig`    | whether to override the default dnsmasq configuration instead of appending to it   | `false`              |
| `dns.servers`            | List of DNS servers to use                                                         | `["8.8.8.8"]`        |
| `dns.customConfig`       | Custom configuration for the dnsmasq service                                       | `""`                 |
