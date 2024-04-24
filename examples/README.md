# Examples

Applying an example and see for what should be sent to Golive:

```shell
kubectl apply -k ./simple
kubectl -n k8s-golive-monitor rollout status deployment k8s-golive-monitor
kubectl -n k8s-golive-monitor logs -l app=k8s-golive-monitor --tail=50 -f
```

Switch to another example and see result of monitoring:
```shell
kubectl apply -k ./simple-namespace
kubectl -n k8s-golive-monitor rollout status deployment k8s-golive-monitor
kubectl -n k8s-golive-monitor logs -l app=k8s-golive-monitor --tail=50 -f
```

Once you have finished:
```shell
kubectl delete -k ./simple
```
