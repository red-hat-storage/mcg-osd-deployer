## CONTRIBUTING

***
**Note**: If you are looking for a way to quickly get the addon up and running in your cluster, follow the steps below.
- Enter the addon directory.
```shell
$ cd mcg-osd-deployer
```
- Create a shell function to deploy required resources.
```shell
$ function installMCGAddon {
    declare -a resources=() # Recent BASH change, refer https://stackoverflow.com/a/28058737.
    resources=("namespace" "secrets" "operatorgroup" "catalogsource" "subscription")
    for resource in "${resources[@]}"; do
        oc create -f "hack/deploy/${resource}.yaml"
    done
}
```
- Run the shell function.
```shell
$ installMCGAddon
```
- [**OPTIONAL**] Build your custom image and push.
```shell
export TAG=$TAG
export IMG=quay.io/$USER/mcg-osd-deployer:$TAG
export BUNDLE_IMG=quay.io/$USER/mcg-osd-deployer:bundle-$TAG
echo "\ngenerate\n" && \
make generate && \
echo "\nmanifests\n" && \
make manifests && \
echo "\ndocker-build\n" && \
make docker-build IMG=$IMG && \
echo "\ndocker-push\n" && \
make docker-push IMG=$IMG && \
echo "\nbundle\n" && \
make bundle IMG=$IMG && \
echo "\nbundle-build\n" && \
make bundle-build BUNDLE_IMG=$BUNDLE_IMG && \
echo "\npush\n" && \
docker push $BUNDLE_IMG
```
- [**OPTIONAL**] Point to the custom image in the CSV.

**Note**: This way of deployment is used in the [`mcg-ms-console`](https://github.com/red-hat-storage/mcg-ms-console)'s upstream CI, and does **not** support remote-writing operations on OCP (please use OSD/ROSA clusters to test that).

***

### Prerequisites
Download the project.

```bash
$ git clone git@github.com:red-hat-storage/mcg-osd-deployer.git
$ cd mcg-osd-deployer
$ go mod tidy
```

Install operator-sdk.

```bash
$ curl -LsS https://github.com/operator-framework/operator-sdk/releases/download/v1.18.1/operator-sdk_linux_amd64 -o /usr/local/bin/operator-sdk
$ chmod +x /usr/local/bin/operator-sdk
```
Install kustomize.

```shell
$ curl -LsS https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/v3.9.1/kustomize_v3.9.1_linux_amd64.tar.gz -o kustomize.tar.gz
$ tar -xvzf kustomize.tar.gz
$ mv kustomize /usr/local/bin
```

### Deployment

#### Run without dependencies.

- Create `redhat-data-federation` namespace.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  labels:
    hive.openshift.io/managed: "true"
  name: redhat-data-federation
```
```shell
$ oc create -f namespace.yaml
```

- Create the environment variables needed to run the operator.

```bash
export SOP_ENDPOINT=test.url
export ALERT_SMTP_FROM_ADDR=test@redhat.com
export ADDON_NAME=mcg-osd
export NOOBAA_CORE_IMAGE=registry.redhat.io/odf4/mcg-core-rhel8@sha256:4ff2d65ea16dd1026fe278a0f8ca920f300dfcee205b4b8ede0ab28be1aa43a6
export NOOBAA_DB_IMAGE=registry.redhat.io/rhel8/postgresql-12@sha256:be7212e938d1ef314a75aca070c28b6433cd0346704d0d3523c8ef403ff0c69e
export ADDON_VARIANT=development
export ADDON_ENVIRONMENT=stage
```

```bash
$ make install
$ make run
```

#### Deploy in bundle format via operator-sdk

- Build the image.

```shell
$ make manifests
$ IMG=quay.io/${USER}/mcg-osd-deployer:${TAG}
$ make docker-build docker-push IMG=$IMG
```

**NOTE**: The quay repository must be public, or the appropriate secrets must be injected into the cluster otherwise.

- Generate bundle manifests and build the bundle.

```shell
$ make bundle IMG=$IMG
$ BUNDLE_IMG=quay.io/${USER}/mcg-osd-deployer:<bundle_image_tag>
$ make bundle-build BUNDLE_IMG=$BUNDLE_IMG
$ docker push $BUNDLE_IMG
```

Create necessary secrets required for the addon.

**Note**: For remote-writing to RHOBS, valid values will need to be provided for the `mcg-osd-prom-remote-write` secret.

```yaml
kind: Secret
apiVersion: v1
metadata:
  name: mcg-osd-deadmanssnitch
  namespace: redhat-data-federation
stringData:
  SNITCH_URL: https://nosnch.in/4a029adb4c
type: Opaque
---
kind: Secret
apiVersion: v1
metadata:
    name: mcg-osd-pagerduty
    namespace: redhat-data-federation
stringData:
    PAGERDUTY_KEY: testKey
type: Opaque
---
apiVersion: v1
data:
    host: c210cC5zZW5kZ3JpZC5uZXQ=
    password: dGVzdC1wYXNzd29yZAo=
    port: NTg3
    tls: dHJ1ZQ==
    username: YXBpa2V5
kind: Secret
metadata:
    name: mcg-osd-smtp
    namespace: redhat-data-federation
type: Opaque
---
kind: Secret
apiVersion: v1
metadata:
    name: addon-mcg-osd-parameters
    namespace: redhat-data-federation
labels:
    hive.openshift.io/managed: 'true'
data:
    notification-email-0: bmFwYXVsQHJlZGhhdC5jb20=
    notification-email-1: bmFwYXVsQHJlZGhhdC5jb20=
    notification-email-2: bmFwYXVsQHJlZGhhdC5jb20=
type: Opaque
---
apiVersion: v1
data:
    prom-remote-write-config-id: [REDACTED]
    prom-remote-write-config-secret: [REDACTED]
    rhobs-audience: [REDACTED]
kind: Secret
metadata:
    name: mcg-osd-prom-remote-write
    namespace: redhat-data-federation
type: Opaque
```

- Create downstream prometheus `CatalogSource`.

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: downstream-prometheus-operator
  namespace: redhat-data-federation
spec:
  displayName: downstream-prometheus-operator
  image: quay.io/dbindra/ose-prometheus-operator:4.10.0_catsrc
  publisher: RedHat
  sourceType: grpc
```

- Add all the yaml files to the cluster.

```shell
$ oc create -f ./hack/deploy/namespace.yaml
$ oc create -f ./hack/deploy/secrets.yaml
$ oc create -f ./hack/deploy/prometheus.yaml
$ oc create -f ./hack/deploy/addonconfigmap.yaml
```

- Deploy the bundle using operator-sdk.
```shell
$ operator-sdk run bundle -n redhat-data-federation $BUNDLE_IMG --index-image=quay.io/operator-framework/opm:v1.23.0 --timeout=5m0s
```

- Create the `ConfigMap` below and add this to `mcg-osd-deployer-controller-manager` deployment `manager` pod.

```yaml
kind: ConfigMap
apiVersion: v1
metadata:
  name: addon-env
  namespace: redhat-data-federation
data:
  ADDON_ENVIRONMENT: stage
  ADDON_NAME: mcg-osd
  ADDON_VARIANT: dev
  ALERT_SMTP_FROM_ADDR: test@redhat.com
  NOOBAA_CORE_IMAGE: registry.redhat.io/odf4/mcg-core-rhel8@sha256:4ff2d65ea16dd1026fe278a0f8ca920f300dfcee205b4b8ede0ab28be1aa43a6
  NOOBAA_DB_IMAGE: registry.redhat.io/rhel8/postgresql-12@sha256:be7212e938d1ef314a75aca070c28b6433cd0346704d0d3523c8ef403ff0c69e
  OPERATOR_CONDITION_NAME: mcg-osd-deployer.v1.0.0
  RHOBS_ENDPOINT: https://observatorium-mst.api.stage.openshift.com/receive
  RH_SSO_TOKEN_ENDPOINT: https://sso.redhat.com/auth/realms/redhat-external/token
  SOP_ENDPOINT: test.url
```

- In your redhat-data-federation namespace you should see the resources below.

```shell
$ oc get all -n redhat-data-federation
NAME                                                                  READY   STATUS      RESTARTS       AGE
pod/94ea90fd31164f877109f8ca4e737a75754e342d9f247d8cc7b61278f6l2nzn   0/1     Completed   0              3h7m
pod/alertmanager-managed-mcg-alertmanager-0                           2/2     Running     0              17m
pod/downstream-prometheus-operator-flzjn                              1/1     Running     0              3h51m
pod/ff120027f4c7a69a749cb3276ca30aeb5c70094cd1d7d7da01aad1bca5hg5rh   0/1     Completed   0              3h7m
pod/mcg-ms-console-6b49b58fc7-z82zt                                   1/1     Running     0              18m
pod/mcg-osd-deployer-controller-manager-5dff796c47-snhpb              3/3     Running     0              18m
pod/noobaa-core-0                                                     1/1     Running     0              16m
pod/noobaa-db-pg-0                                                    1/1     Running     0              16m
pod/noobaa-default-backing-store-noobaa-pod-9c20bab9                  1/1     Running     0              14m
pod/noobaa-endpoint-5957598449-tzss6                                  1/1     Running     0              14m
pod/noobaa-operator-7cb858bd6c-n7k7z                                  1/1     Running     0              17m
pod/prometheus-managed-mcg-prometheus-0                               3/3     Running     0              17m
pod/prometheus-operator-6446764487-xlrsz                              1/1     Running     0              18m
pod/quay-io-jmolmo-mcg-osd-deployer-bundle                            1/1     Running     5 (3h9m ago)   3h11m

NAME                                                          TYPE           CLUSTER-IP       EXTERNAL-IP                                                               PORT(S)                                                    AGE
service/alertmanager-operated                                 ClusterIP      None             <none>                                                                    9093/TCP,9094/TCP,9094/UDP                                 17m
service/downstream-prometheus-operator                        ClusterIP      172.30.42.155    <none>                                                                    50051/TCP                                                  3h51m
service/mcg-ms-console-service                                ClusterIP      172.30.5.86      <none>                                                                    9002/TCP                                                   17m
service/mcg-osd-deployer-controller-manager-metrics-service   ClusterIP      172.30.142.187   <none>                                                                    8443/TCP                                                   18m
service/noobaa-db-pg                                          ClusterIP      172.30.61.232    <none>                                                                    5432/TCP                                                   16m
service/noobaa-mgmt                                           LoadBalancer   172.30.44.201    a562f035bb8ff49bebd1e64c879ca3db-1720663684.eu-west-2.elb.amazonaws.com   80:30782/TCP,443:30509/TCP,8445:31521/TCP,8446:30978/TCP   16m
service/noobaa-operator-service                               ClusterIP      172.30.239.87    <none>                                                                    443/TCP                                                    17m
service/prometheus                                            ClusterIP      172.30.43.85     <none>                                                                    9339/TCP                                                   17m
service/prometheus-operated                                   ClusterIP      None             <none>                                                                    9090/TCP                                                   17m
service/s3                                                    LoadBalancer   172.30.90.99     a60e57ce48efd492b827a3437e26f72d-965410249.eu-west-2.elb.amazonaws.com    80:32275/TCP,443:31868/TCP,8444:32318/TCP,7004:32430/TCP   16m

NAME                                                  READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/mcg-ms-console                        1/1     1            1           18m
deployment.apps/mcg-osd-deployer-controller-manager   1/1     1            1           18m
deployment.apps/noobaa-endpoint                       1/1     1            1           14m
deployment.apps/noobaa-operator                       1/1     1            1           17m
deployment.apps/ocs-metrics-exporter                  0/0     0            0           17m
deployment.apps/ocs-operator                          0/0     0            0           17m
deployment.apps/prometheus-operator                   1/1     1            1           18m
deployment.apps/rook-ceph-operator                    0/0     0            0           17m

NAME                                                             DESIRED   CURRENT   READY   AGE
replicaset.apps/mcg-ms-console-6b49b58fc7                        1         1         1       18m
replicaset.apps/mcg-osd-deployer-controller-manager-5dff796c47   1         1         1       18m
replicaset.apps/noobaa-endpoint-5957598449                       1         1         1       14m
replicaset.apps/noobaa-operator-7cb858bd6c                       1         1         1       17m
replicaset.apps/ocs-metrics-exporter-b599f747c                   0         0         0       17m
replicaset.apps/ocs-operator-5fbcf4fdc9                          0         0         0       17m
replicaset.apps/prometheus-operator-6446764487                   1         1         1       18m
replicaset.apps/rook-ceph-operator-8d74957f                      0         0         0       17m

NAME                                                     READY   AGE
statefulset.apps/alertmanager-managed-mcg-alertmanager   1/1     17m
statefulset.apps/noobaa-core                             1/1     16m
statefulset.apps/noobaa-db-pg                            1/1     16m
statefulset.apps/prometheus-managed-mcg-prometheus       1/1     17m

NAME                                                  REFERENCE                    TARGETS   MINPODS   MAXPODS   REPLICAS   AGE
horizontalpodautoscaler.autoscaling/noobaa-endpoint   Deployment/noobaa-endpoint   0%/80%    1         2         1          14m

NAME                                                                        COMPLETIONS   DURATION   AGE
job.batch/94ea90fd31164f877109f8ca4e737a75754e342d9f247d8cc7b61278f64c5fa   1/1           36s        3h7m
job.batch/ff120027f4c7a69a749cb3276ca30aeb5c70094cd1d7d7da01aad1bca5db4bc   1/1           36s        3h7m

NAME                                   HOST/PORT                                                             PATH   SERVICES      PORT         TERMINATION          WILDCARD
route.route.openshift.io/noobaa-mgmt   noobaa-mgmt-redhat-data-federation.apps.juanmi.j7wh.s1.devshift.org          noobaa-mgmt   mgmt-https   reencrypt/Redirect   None
route.route.openshift.io/s3            s3-redhat-data-federation.apps.juanmi.j7wh.s1.devshift.org                   s3            s3-https     reencrypt/Allow      None
```
### Uninstallation

The uninstallation flow will start when user clicks on the uninstallation button in addon tab in the OCM UI. The uninstallation flow will create a `ConfigMap` with the same name as addon, and the operator will start deleting addon specific resources once this `ConfigMap` is created in the namespace.
```yaml
kind: ConfigMap
apiVersion: v1
metadata:
  name: mcg-osd
  namespace: redhat-data-federation
labels:
  api.openshift.com/addon-mcg-osd-delete: 'true'
  hive.openshift.io/managed: 'true'
```

### SendGrid Configuration

- [Sendgrid Onboarding](https://gitlab.cee.redhat.com/service/ocm-sendgrid-service#addon-onboarding-process)
- [add Data Federation SMTP secret](https://gitlab.cee.redhat.com/service/app-interface/-/merge_requests/37022)
- [add Data Federation addon to SendGrid Service Webhook](https://gitlab.cee.redhat.com/service/app-interface/-/merge_requests/37193)

### OpenShift CI PR

[Enable upstream pipeline for the addon](https://github.com/openshift/release/pull/27907)
[Enable upstream pipeline for the UI plugin](https://github.com/openshift/release/pull/28265)

### Enable core.hooksPath support

Enable hooks: `$ git config --local core.hooksPath hack/hooks/`

Since hooks are not pushed by git, it is recommended to enable hooks path voluntarily to run appropriate hooks and profit from a more convenient development process.

**NOTE**: Your git version needs to be >=2.9.0 for this to work.

### Accessing NooBaa Bucket data

Prerequisite: Create an S3 bucket in AWS.

#### OSD/ROSA Cluster

- Install the addon.
- List all the services in `redhat-data-federation` namespace. For Data Federation, NooBaa S3 service will provide S3 Bucket access.

```
s3  LoadBalancer   172.30.89.163    a4718**0340093.us-east-1.elb.amazonaws.com   80:31299/TCP,443:32666/TCP,8444:32282/TCP,7004:31323/TCP
```

- You can operate on the S3 bucket using the following commands.

```shell
$ NOOBAA_ACCESS_KEY=$(kubectl get secret noobaa-admin -n redhat-data-federation -o json | jq -r '.data.AWS_ACCESS_KEY_ID|@base64d')
$ NOOBAA_SECRET_KEY=$(kubectl get secret noobaa-admin -n redhat-data-federation -o json | jq -r '.data.AWS_SECRET_ACCESS_KEY|@base64d')

alias s3='AWS_ACCESS_KEY_ID=$NOOBAA_ACCESS_KEY'

# Create S3 alies for the bucket, replace the placeholder S3_service_url below with the service endpoint.
AWS_SECRET_ACCESS_KEY='$NOOBAA_SECRET_KEY aws --endpoint https://{{S3_service_url}} --no-verify-ssl s3'

# List all the noobaa S3 bucket.
s3 ls

# List content in noobaa bucket.
s3 ls s3://{{noobaa_bucket_name}}

# Copy data to the bucket, this will only copy to the write only S3 bucket.
s3 cp {{data}} s3://{{noobaa_bucket_name}}
```



