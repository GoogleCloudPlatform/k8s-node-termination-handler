### This is not an official Google Project

# Kubernetes on GCP Node Termination Event Handler

This repo provides a solution for translating GCE node termination events to graceful pod terminations in Kubernetes.
GCE VMs are typically live migratable.
However, Preemptible VMs and VMs with Accelerators are not live migratable and are hence prone to VM terminations.
*Do not consume this solution* unless you are managing k8s clusters that run non migratable VM types.

**Under Construction**
