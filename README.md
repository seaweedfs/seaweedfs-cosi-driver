# s3gw-cosi-driver

COSI driver implementation for [s3gw](https://github.com/s3gw-tech/s3gw).

Note that the COSI driver alone is not sufficient to get COSI working
on a Kubernetes cluster.

You can deploy a full COSI installation for s3gw following the instructions for the
[s3gw's Helm charts](https://s3gw-docs.readthedocs.io/en/latest/helm-charts/).

## Building

the driver code can be compiled using:

```shell
make build
```

Now build the docker image and provide a tag as `quay.io/s3gw/s3gw-cosi-driver:latest`

```shell
make container
```

You can tag and push the docker image to a registry with:

```shell
make push REGISTRY_NAME=quay.io/s3gw
```

## Examples

### Create BucketClaim, BucketAccess and consuming the claim in a pod

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
