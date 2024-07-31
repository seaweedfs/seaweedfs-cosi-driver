# seaweedfs-cosi-driver

COSI driver implementation for [SeaweedFS](https://github.com/chrislusf/seaweedfs).

Note that the COSI driver alone is not sufficient to get COSI working
on a Kubernetes cluster.

You can deploy a full COSI installation for SeaweedFS following the instructions for the
[SeaweedFS's Helm charts](https://github.com/seaweedfs/seaweedfs/tree/master/k8s/charts/seaweedfs).

## Building

The driver code can be compiled using:

```shell
make build
```

Now build the docker image and provide a tag as `quay.io/seaweedfs/seaweedfs-cosi-driver:latest`

```shell
make container
```

You can tag and push the docker image to a registry with:

```shell
make push REGISTRY_NAME=quay.io/seaweedfs
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
    "name": "bc-9b55f9f1-2492-4d41-a380-09f9a32e85ed",
    "creationTimestamp": null
  },
  "spec": {
    "bucketName": "sample-bcc5e103d90-f638-412a-9c5a-79e3e51fe4f0",
    "authenticationType": "KEY",
    "secretS3": {
      "endpoint": "http://seaweedfs.seaweedfs.svc.cluster",
      "region": "",
      "accessKeyID": "3TQDWKY2JJ4W8TAQJKCG",
      "accessSecretKey": "k2NrXAPLFMCjHtsPJCjV4QWSNzSIHDHEA8BT9xaZ"
    },
    "secretAzure": null,
    "protocols": [
      "s3"
    ]
  }
}
```
