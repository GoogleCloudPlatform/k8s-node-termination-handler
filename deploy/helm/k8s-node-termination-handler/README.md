# k8s-node-termination-handler-helm-chart
A solution to gracefully handle GCE VM terminations in kubernetes clusters

## Introduction
This chart bootstraps an [adapter for translating GCE node termination events to graceful pod terminations in Kubernetes](https://github.com/GoogleCloudPlatform/k8s-node-termination-handler) deployment on a Kubernetes cluster using the Helm package manager.

## Prerequisites
* Kubernetes 1.11+
* Helm 3.0.2+

## Parameters
| Parameter                 | Description                      | Default                                                          | Aditional notes                                                                         |
| ---                       |  ---                             | ---                                                              | ---                                                                                     | 
| `gke_accelerator.enabled` | GKE node GPU accelerator enabled | false                                                            | -                                                                                       |
| `gke_preemptible.enabled` | GKE preemtible nodes enabled     | true                                                             | -                                                                                       |
| `image.repository`        | image name                       | k8s.gcr.io/gke-node-termination-handler@sha256                   | -                                                                                       |
| `image.tag`               | image tag                        | aca12d17b222dfed755e28a44d92721e477915fb73211d0a0f8925a1fa847cca | -                                                                                       |
| `image.pullPolicy`        | Image pull policy                | IfNotPresent                                                     | -                                                                                       |
| `rbac.pspEnabled`         | If true, create and use a restricted pod security policy        | true                                                                | -                                                                                       |
| `resources.limits.cpu`    | cpu limit                        | 150m                                                             | -                                                                                       |
| `resources.limits.memory` | memory limit                     | 30Mi                                                             | -                                                                                       |
| `slack`                   | slack webhook                    | -                                                                | This functionality has been added to the project recently. We are currently testing it. |
| `updateStrategy`          | Set up update strategy           | RollingUpdate                                                    | -                                                                                       |

## Installing the Chart

```
helm install <my-release> k8s-node-termination-handler-helm-chart --namespace <namespace> --set psp_reourceName=<podSecurityPolicy-name>
```
