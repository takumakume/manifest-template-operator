# manifest-template-operator

An operator that generates any CRD using go-template.

## Sample manifest

### `ManifestTemplate`

```yaml
apiVersion: manifest-template.takumakume.github.io/v1alpha1
kind: ManifestTemplate
metadata:
  name: sample-svc
  namespace: test
spec:
  apiVersion: v1
  kind: Service
  metadata:
    name: sample-svc
    namespace: "{{ .Self.ObjectMeta.Namespace }}"
    labels:
      label1: label1value
    annotations:
      annotation1: annotation1value
  spec:
    ports:
    - name: "http"
      port: 80
    selector:
      app: test1
      ns: "{{ .Self.ObjectMeta.Namespace }}"
```

### Generated resource

```yaml
apiVersion: v1
kind: Service
metadata:
  name: sample-svc
  namespace: test
  annotations:
    annotation1: annotation1value
    ns: test
  labels:
    label1: label1value
    ns: test
  ownerReferences:
  - apiVersion: manifest-template.takumakume.github.io/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: ManifestTemplate
    name: sample
    uid: "xxx"
spec:
  ports:
  - name: http
    port: 80
  selector:
    app: test1
    ns: test
```

## Variables for go-template

### `.Self`

The variable `.Self` used in the template is the ManifestTemplate resource itself.

For example, if you need the namespace where the ManifestTemplate is deployed, you can access it like `.Self.Metadata.Namespace` .

## Use case

- Embed namespace identifier when deploying the same manifests to multiple namespaces
  - Embed namespace name in part of hostname such as Ingress, HTTPRoute resources
    ```yaml
    # example: Ingress resource template
    ---
    apiVersion: manifest-template.takumakume.github.io/v1alpha1
    kind: ManifestTemplate
    metadata:
      name: sample-ingress
      namespace: test
    spec:
      apiVersion: networking.k8s.io/v1
      kind: Ingress
      metadata:
        name: example
        namespace: test
      spec:
      tls:
      - hosts:
        - "app-{{ .Self.ObjectMeta.Namespace }}.example.com"
        secretName: example-com-tls
      rules:
      - host: "app-{{ .Self.ObjectMeta.Namespace }}.example.com"
        http:
        paths:
        - backend:
          service:
            name: example
            port:
              number: 80
          path: /
          pathType: Prefix
    ```
    ```yaml
    # example: ManifestTemplate resource template
    ---
    apiVersion: manifest-template.takumakume.github.io/v1alpha1
    kind: ManifestTemplate
    metadata:
      name: sample-httproute
      namespace: test
    apiVersion: gateway.networking.k8s.io/v1beta1
    kind: HTTPRoute
    metadata:
      name: httproute
    spec:
      parentRefs:
      - name: gateway
        namespace: istio-system
      hostnames: ["app-{{ .Self.ObjectMeta.Namespace }}.example.com"]
      rules:
      - matches:
        - path:
            type: PathPrefix
            value: /
        backendRefs:
        - name: sample-app
          port: 80
    ```

## Install

### Using Helm

```sh
$ helm repo add manifest-template-operator https://takumakume.github.io/manifest-template-operator/charts
$ helm repo update
$ helm install --create-namespace --namespace manifest-template-operator-system manifest-template-operator manifest-template-operator/manifest-template-operator
```

Values: ref [charts/manifest-template-operator/values.yaml](https://github.com/takumakume/manifest-template-operator/blob/main/charts/manifest-template-operator/values.yaml)
