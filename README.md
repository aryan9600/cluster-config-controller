# cluster-conifg-controller

## Introduction
cluster-config-controller is a tool (a Kubernetes operator) to manage ConfigMaps cluster-wide.

## Usage
> Please make sure you have a Kubernetes cluster running before using the operator.

* Clone the repository.
* To install the CRD (`ClusterConfigMap`) into the cluster: `make install`
* To run the controller: `make run ENABLE_WEBHOOKS=false`

### Using Docker
* `make deploy aryan9600/cluster-config-controller:latest`

---
* Create a Namespace:
    ```yaml
    # ns1.yaml
    apiVersion: v1
    kind: Namespace
    metadata:
        name: ns1
        labels:
            app.kubernetes.io/managed-by: "dev-team"
    ```
   `kubectl apply -f ns1.yaml` 
   
* Create a ClusterConfigMap object: `kubectl apply -f example/ccm.yaml`
