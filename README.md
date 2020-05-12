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

- If targeted VM is Preemptible VM, specify `n` with a value from `0s` to `15s`.
- If targeted VM is regular VM, specify `n` with a value from `0s` to the value of `--regular-vm-timeout/2`.

If you follow the rules above, `VM timeout - system-grace-pod-period` will be given as a grace period for deleting regular pods.
