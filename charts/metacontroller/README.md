
# Getting started

1. Create a clusterrolebinding for your user account:

```
kubectl create clusterrolebinding <user>-cluster-admin-binding --clusterrole=cluster-admin --user=<user>@<domain>
```

2. Install this chart:

```
helm install --name metacontroller --namespace metacontroller .
```