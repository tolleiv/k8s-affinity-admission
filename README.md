# Kubernetes Admission Webhook for Affinity patches

This Kubernetes Admission checks and eventually adds pod affinity configuration to pods.

## Attribution

This projects was heavily inspired by the work and examples found in the following repos:
* https://github.com/lachie83/vk-affinity-admission-controller
* https://github.com/kubernetes/kubernetes/tree/release-1.10/test/images/webhook

## Supported Kubernetes versions

* 1.10+

## Prerequisites
Please enable the admission webhook feature
[doc](https://kubernetes.io/docs/admin/extensible-admission-controllers/#enable-external-admission-webhooks).

## Configuration

Parameters:
* `--mode` - configures the working mode
* `--affinityPatch` [v1.Affinity](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#affinity-v1-core) configuration serialized to JSON
* `--podSelector` [metaV1.LabelSelector](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#labelselector-v1-meta) selector to specify which Pods to patch serialized to JSON

Supported working modes are:

* ``patchMissing`` (default) will apply the ``affinityPatch`` to all Pods which have no affinity configuration
* ``denyMissing`` will prevent the creation of Pods without affinity configuration
* ``patchAlways`` will apply the ``affinityPatch`` to all Pods

## Deployment through Helm

The deployment can be greatly simplified using Helm which configures the deployment, registers the webhooks and takes care of transforming the affinityPatch from Yaml to Json.

```
helm install --name admission-webhook local/chart
```

### Chart configuration

The following table lists the configurable parameters of the chart and their default values.

Parameter | Description | Default
--------- | ----------- | -------
`rbac.create` | If true, create & use RBAC resources | `true`
`admissionRegistration.failurePolicy` | FailurePolicy defines how unrecognized errors from the admission endpoint are handled - allowed values are Ignore or Fail. Defaults to Ignore. | `Ignore`
`admissionRegistration.namespaceSelector` | [LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#labelselector-v1-meta) which decides whether to run the webhook on an object based on whether the namespace for that object matches the selector. |  `matchLabels: {affinity-injection: enabled}`
`controller.image.repository`  | Container image repository | `tolleiv/k8s-affinity-admission`
`controller.image.tag` | Container image repository | `latest`
`controller.image.pullPolicy` | Container image pull policy | `Always`
`controller.replicaCount` | Controller replica count | `1`
`controller.affinity` | Affinity configuration for the controller pod |  `{}`
`controller.tolerations` | Toleration labels for pod assignment | `[]` |
`controller.nodeSelector` | Node labels for pod assignment | `{}` |
`controller.podAnnotations` | Pod annotations | `{}` |
`controller.deploymentAnnotations` | Deployment annotations | `{}` |
`controller.podAnnotations` | Deployment annotations | `{}` |
`controller.args.mode` | Possible values: denyMissing, patchMissing, patchAlways - see above | `patchMissing`
`controller.args.affinityPatch` | The affinity patch to apply (Yaml is rendered to Json through the template) - see above | `{}`
`controller.args.podSelector` | The podSelector for filtering (Yaml is rendered to Json through the template) - see above | `{}`
`controller.args.verbosity` | Logging verbosity level | `4` |
`controller.serviceAccount` | Name of the service account to use or create | `k8s-affinity-admission-controller` 
`controller.tls.requestHeaderCA` | Admission controller server will inherit this CA from the extension-apiserver-authentication ConfigMap if available | `""`
`controller.service.type` | Type of service to create | `NodePort`
`controller.service.port` | Server service port | `443`
`controller.service.targetPort` | Port the controller should use | `8443`


## Chart values example

The following example configuration for the Helm chart would make sure that the controller Pod runs on nodes labeled `security=S1` while it would patch all Pods without affinity configuration to run on nodes labels `security=S2`.

```
$ cat << EOF > values.yaml
admissionRegistration:
  namespaceSelector:
    matchLabels:
      affinity-injection: enabled

controller:
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: core
            operator: In
            values:
            - "true"

  args:
    mode: patchMissing
    affinityPatch:
      nodeAffinity:
        requiredDuringSchedulingIgnoredDuringExecution:
          nodeSelectorTerms:
          - matchExpressions:
            - key: core
              operator: In
              values:
              - "false"
    podSelector:
      matchExpressions:
        - {key: app, operator: NotIn, values: [backend,database]}
EOF

$ helm install --name admission-webhook --values values.yaml local/chart

$ kubectl label namespace default affinity-injection=enabled
```
