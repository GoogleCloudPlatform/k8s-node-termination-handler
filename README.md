> ## :warning: Deprecation Notice
> As of Kubernetes 1.20, Graceful Node Shutdown replaces the need for GCP Node termination handler.
> GKE on versions 1.20+ enables Graceful Node Shutdown by default.
> Refer to the [GKE documentation](https://cloud.google.com/kubernetes-engine/docs/how-to/preemptible-vms#kubernetes_preemptible_nodes) and Kubernetes documentation for more info about Graceful Node Shutdown ([docs](https://kubernetes.io/docs/concepts/architecture/nodes/#graceful-node-shutdown), [blog post](https://kubernetes.io/blog/2021/04/21/graceful-node-shutdown-beta/)).

*This is not an official Google Project*

# Kubernetes on GCP Node Termination Event Handler

This project provides an adapter for translating GCE node termination events to graceful pod terminations in Kubernetes.
GCE VMs are typically live migratable. However, Preemptible VMs and VMs with Accelerators are not live migratable and are hence prone to VM terminations.
Do not consume this project unless you are managing k8s clusters that run non migratable VM types.

To deploy this solution to a GKE or a GCE cluster:
```shell
kubectl apply -f deploy/
```

**Note**: This solution requires kubernetes versions >= 1.11 to work on Preemptible nodes.

The app deployed as part of this solution does the following:

1. Launch a pod on every node in the cluster which contains the node termination monitoring agent.
2. The agent in the pod watches for node terminations via GCE metadata APIs.
3. Whenever a termination event is observed, the agent does the following:
   1. Taints the node to prevent new pods from being scheduled
   2. Delete all pods that are not in the `kube-system` namespace first before deleting the ones in it. Certain system pods like logging agents might need more time to flush out logs prior to termination and for this reason, pods in `kube-system` namespaces are deleted last.
   3. Reboot the node if the underlying VM is not a preemptible VM. VMs with Accelerators when restarted are expected to handle host maintenance events transparently. Restarts are generally faster too!
4. If the underlying node is not scheduled for maintenance, the agent will remove any previously applied taints, thereby restoring the node post termination.

The agent crashes whenever it encounters an unrecoverable error with the metadata APIs.
This agent is not production hardened yet and so use it with caution.

## Graceful terminations for regular pods (Non-system pods)

The pods that are not in the kube-system are called **regular pods** in this agent.
By default, regular pods are deleted immediately before deleting system pods.
If you want to delete regular pods gracefully, please add `--system-pod-grace-period=n` in arguments according to the following rules:

- If targeted VM is Preemptible VM, specify `n` with a value from `0s` to `14s`.
- If targeted VM is regular VM, specify `n` with a value from `0s` to the value of `(--regular-vm-timeout / 2) - 1`.

If you follow the rules above, `VM timeout - system-grace-pod-period` will be given as a grace period for deleting regular pods.
Note that `VM timeout` in Preemptible VM is [30 seconds](https://cloud.google.com/compute/docs/instances/preemptible#preemption-process).

If you specify `0s`, the system pods will be terminated immediately and the regular pods will have about 30 seconds of grace period.
If you specify `14s`, both system and regular pods will have about `14s` of grace period.

Also, `the timeout value of VM (e.g. preemptible=30s) / 2` cannot be used as a maximum value in `--system-pod-grace-period` for regular pods.

In addition, if the actual delete process fails, it will retry internally based on exponential backoff. In that case, the grace period is set considering the elapsed time, but it may shorten the actual grace period.
