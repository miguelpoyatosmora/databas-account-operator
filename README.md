# database-account-operator
PostgreSQL database-account operator

## Description
We are having 1 standalone PostgreSQL server running in Kubernetes as a prerequirement.
Create a Kubernetes operator that interacts with that PostgreSQL server, having admin rights
and it having the following responsibilities:
1. When custom Database CRD is created, connect to the instance and ensure the
Database is created.
2. When a custom User CRD is created, create DB users
3. When a custom Grant CRD is created, assign permissions on User to Database

Add Status fields in both CRD to indicate what’s the status of the underlying operation.

Database CRD should specify and use following:
- ENCODING (optional)
- LC_COLLATE (optional)
- LC_TYPE (optional)

Account CRD should specify and use following:
- NAME
- PASSWORD
- VALID_UNTIL (optional)

Grant CRD should specify and use following:
- GRANT_TYPE /all, insert, create, delete, update or combination/
- TO
- DATABASE

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster
1. Install postgres withing the cluster

2. Edit the file `config/samples/database-account-operator_v1_postgresqldatabase.yaml` and configure the `spec.address` field to point to you postgres installation

3. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/
```

4. Build and push your image to the location specified by `IMG`:
	
```sh
make docker-build docker-push IMG=<some-registry>/database-account-operator:tag
```
	
5. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/database-account-operator:tag
```

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller to the cluster:

```sh
make undeploy
```

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/) 
which provides a reconcile function responsible for synchronizing resources untile the desired state is reached on the cluster 

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

