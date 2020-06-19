# Dell EMC Isilon CSI Driver
## TL;DR;

Add the repo (if you haven't already):
```bash
$ helm repo add isilon https://isilon.github.io/charts
```

Install the driver using helm 2:
```bash
$ helm install --values myvalues.yaml --name isilon-csi --namespace isilon ./csi-isilon
```

Install the driver using helm 3:
```bash
$ helm install isilon --values myvalues.yaml --namespace isilon ./csi-isilon
```

## Introduction

This chart bootstraps the Isilon driver on a [Kubernetes](http://kubernetes.io) cluster using the [Helm](https://helm.sh) package manager.

## Prerequisites

- Kubernetes 1.13 or later with feature gates as enabled in the release instructions.
- Isilon REST API gateway (with approved Isilon certificate)
- You must configure a Kubernetes secret containing the Isilon username and password.

## Installing the Chart

To install the chart with the release name `isilon` using helm 2:

```bash
$ helm install --values myvalues.yaml --name isilon --namespace isilon ./csi-isilon
```
> **Tip**: List all releases using `helm list`

To install the chart with the release name `isilon` using helm 3:

```bash
$ helm install isilon --values myvalues.yaml --namespace isilon ./csi-isilon
```

There are a number of required values that must be set either via the command-line or a [`values.yaml`](values.yaml) file. Those values are listed in the configuration section below.

## Uninstalling the Chart

To uninstall/delete the `my-release` deployment using helm 2:

```bash
$ helm delete isilon [--purge]
```

The command removes all the Kubernetes components associated with the chart and deletes the release. The purge option also removes the provisioned release name, so that the name itself can also be reused.

To uninstall/delete the `my-release` deployment using helm 3:

```bash
$ helm delete -n isilon isilon
```

## Configuration

The following table lists the primary configurable parameters of the Isilon driver chart and their default values. More detailed information can be found in the [`values.yaml`](values.yaml) file in this repository.

| Parameter | Description | Required | Default |
| --------- | ----------- | -------- |-------- |
| systemName | Name of the Isilon system   | true | - |
| restGateway | REST API gateway HTTPS endpoint Isilon system | true | - |
| controllerCount | Number of driver controllers to create | false | 1 |
| storageClass.name | Name of the storage class to be defined | false | isilon |
| storageClass.isDefault | Whether or not to make this storage class the default | false | true |
| storageClass.reclaimPolicy | What should happen when a volume is removed | false | Delete |

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install` or in a myvalues.yaml file. For example,

```bash
$ helm install --name isilon-csi --namespace isilon \
  --set systemName=isilon_sys,restGateway=https://123.0.0.1 \
    isilon/isilon-csi
```
Alternatively, a YAML file that specifies the values for the parameters can be provided while installing the chart. For example,

```bash
$ helm install --name isilon -f values.yaml isilon/csi-isilon
```

```yaml
# values.yaml

systemName: isilon
restGateway: 123.0.0.1
```

> **Tip**: You can add required parameters and then use the default [`values.yaml`](values.yaml)
