# Amazon NLB Ingress Controller

## Getting Started

The default configuration assumes you are using kube2iam to manage pod permissions.
To set up a role for this controller use the following command

```sh
export INSTANCE_ROLE_ARNS=`comma delimited list of k8s worker instance ARNs`
make iam
```

To build and deploy the controller

```sh
export IMG=`some ecr repository`
export IAMROLEARN=`the iam role arn created above`

make docker-build
make docker-push
make deploy
```



## Example

```yaml
---
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  annotations:
    kubernetes.io/ingress.class: nlb
  name: sample
  namespace: default
spec:
  rules:
  - http:
      paths:
      - backend:
          serviceName: bookservice
          servicePort: 80
        path: /api/book
      - backend:
          serviceName: authorservice
          servicePort: 80
        path: /api/author
```
