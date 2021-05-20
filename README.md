# A reconciler that mangers other reconcilers

A controller that spins up other managers that watch for types dynamically.
These dynamic types are specified in the `Monitor` resource via their GVK.

## Test

1. Setup Controller `make install && make run`
2. Install RMQ
```bash
kubectl apply -f https://github.com/rabbitmq/cluster-operator/releases/latest/download/cluster-operator.yml`
```
3. Apply a monitor on RMQ's GVK
```bash
kubectl apply -f config/samples/monitor.yaml
```
4. Observe controller logs state a new manager has spawned
5. Create a RMQ instance
```bash
kubectl apply -f https://raw.githubusercontent.com/rabbitmq/cluster-operator/main/docs/examples/hello-world/rabbitmq.yaml
```
6. Observe the controller logs show a reconcile for RMQ instance `hello-world`
7. Delete monitor
```bash
kubectl delete -f config/samples/monitor.yaml
```
8. Observe the controller logs show the manager has been cancelled.


## Note

More information on why we need to manager at the controller-runtime Manager level can be found in this excellent write up by Glyn Normington https://raw.githubusercontent.com/vmware-labs/reconciler-runtime/fdf2f9ace44e739046539db47d3eca01dba3f71a/informers/doc.go and related [issue](https://github.com/vmware-labs/reconciler-runtime/pull/67).
