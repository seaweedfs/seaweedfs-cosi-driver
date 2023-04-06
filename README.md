# s3gw-cosi-driver

COSI driver implementation for [s3gw](https://github.com/aquarist-labs/s3gw).

## Installing CRDs and the COSI controller

```shell
kubectl create -k github.com/kubernetes-sigs/container-object-storage-interface-api
kubectl create -k github.com/kubernetes-sigs/container-object-storage-interface-controller
```

Check for the controller pod in the default namespace:

```shell
NAME                                        READY   STATUS    RESTARTS   AGE
objectstorage-controller-6fc5f89444-4ws72   1/1     Running   0          2d6h
```

## Building

the driver's code can be compiled using:

```shell
make build
```

Now build the docker image and provide a tag as `ghcr.io/giubacc/s3gw-cosi-driver:latest`

```shell
$ make container
Sending build context to Docker daemon  41.95MB
```

You can tag and push the docker image to a registry with:

```shell
docker tag s3gw-cosi-driver:latest ghcr.io/giubacc/s3gw-cosi-driver:latest
docker push ghcr.io/giubacc/s3gw-cosi-driver:latest
```

## Installing with Helm

Now install the sidecar and the s3gw's COSI driver with:

```shell
helm install s3gw-cosi charts/s3gw-cosi
```

Check the driver pod:

```shell
$ kubectl -n s3gw-cosi-driver get pods

NAME                                         READY   STATUS    RESTARTS   AGE
objectstorage-provisioner-6c8df56cc6-lqr26   2/2     Running   0          26h
```

## Create BucketClaim, BucketAccess and consuming the claim in a pod

```shell
kubectl apply -f examples/bucketclass.yaml
kubectl apply -f examples/bucketclaim.yaml
kubectl apply -f examples/bucketaccessclass.yaml
kubectl apply -f examples/bucketaccess.yaml
```

In a pod definition, the bucket claim can be consumed as volume mount:

```yaml
spec:
  containers:
      volumeMounts:
        - name: cosi-secrets
          mountPath: /data/cosi
  volumes:
    - name: cosi-secrets
      secret:
        secretName: ba-secret
```

In the container, at the path: `/data/cosi`, you will find a
file named: `BucketInfo` containing a json:

```json
{
  "metadata": {
    "name": "bc-ceb3a749-b578-4da7-8ea3-607c40093060",
    "creationTimestamp": null
  },
  "spec": {
    "bucketName": "sample-bccf98111be-2edb-402e-a95e-628e178f2818",
    "authenticationType": "KEY",
    "secretS3": {
      "endpoint": "http://s3gw.s3gw.svc.cluster.local",
      "region": "US",
      "accessKeyID": "N7DFI9CCZWZ6QJXI5V1O",
      "accessSecretKey": "2RjtQa3JqPQKVQPf2ux4v8xtdszL8bNtsfna8vV0"
    },
    "secretAzure": null,
    "protocols": [
      "s3"
    ]
  }
}
```
