# Examples

You can apply the following example with [kubectl kustomize](https://kubernetes.io/docs/reference/kubectl/generated/kubectl_kustomize/)
on your cluster:

Once you have finished:
```shell
kubectl delete -k ./simple
```

## Simple
One of the most simple configuration to delegate most of the data extraction to the operator
default policies.

```shell
kubectl apply -k ./simple
kubectl -n k8s-golive-monitor rollout status deployment k8s-golive-monitor
kubectl -n k8s-golive-monitor logs -l app=k8s-golive-monitor --tail=50 -f
```

## Simple Namespace Status
Let's share status of your environments with Golive & Jira Users.
And as it's common to use namespace to categorize environment, let's extract information from it. 

```shell
kubectl apply -k ./simple-namespace-status
kubectl -n k8s-golive-monitor rollout status deployment k8s-golive-monitor
kubectl -n k8s-golive-monitor logs -l app=k8s-golive-monitor --tail=50 -f
```

## Advanced Expressions
You need more power to name your application, category, version, environment, let's see some examples
of advanced expression which can be used to extract information.

```shell
kubectl apply -k ./advanced-expressions
kubectl -n k8s-golive-monitor rollout status deployment k8s-golive-monitor
kubectl -n k8s-golive-monitor logs -l app=k8s-golive-monitor --tail=50 -f
```

## Selectors
Here, we will see selectors which can be useful to restrict the subset of environments you want to keep track of
or if you want to apply specific data extraction policies to specific resources.

```shell
kubectl apply -k ./selectors
kubectl -n k8s-golive-monitor rollout status deployment k8s-golive-monitor
kubectl -n k8s-golive-monitor logs -l app=k8s-golive-monitor --tail=50 -f
```

