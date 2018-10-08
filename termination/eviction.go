// Copyright 2017 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package termination

import (
	"time"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	client "k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
)

const (
	systemNamespace = "kube-system"
	eventReason     = "NodeTermination"
)

type podEvictionHandler struct {
	client               corev1.CoreV1Interface
	node                 string
	recorder             record.EventRecorder
	systemPodGracePeriod time.Duration
}

// List all pods on the node
// Evict all pods on the node not in kube-system namespace
// Return nil on success
func NewPodEvictionHandler(node string, client *client.Clientset, recorder record.EventRecorder, systemPodGracePeriod time.Duration) PodEvictionHandler {
	return &podEvictionHandler{
		client:               client.CoreV1(),
		node:                 node,
		recorder:             recorder,
		systemPodGracePeriod: systemPodGracePeriod,
	}
}

func (p *podEvictionHandler) EvictPods(excludePods map[string]string, timeout time.Duration) error {
	options := metav1.ListOptions{FieldSelector: fields.OneTermEqualSelector("spec.nodeName", string(p.node)).String()}
	pods, err := p.client.Pods(metav1.NamespaceAll).List(options)
	if err != nil {
		glog.V(2).Infof("Failed to list pods - %v", err)
		return err
	}
	var systemPods, regularPods []v1.Pod
	// Separate pods in kube-system namespace such that they can be evicted at the end.
	// This is especially helpful in scenarios like reclaiming logs prior to node termination.
	for _, pod := range pods.Items {
		if ns, exists := excludePods[pod.Name]; !exists || ns != pod.Namespace {
			if pod.Namespace == systemNamespace {
				systemPods = append(systemPods, pod)
			} else {
				regularPods = append(regularPods, pod)
			}
		}
	}
	// Evict regular pods first.
	var gracePeriod int64
	// Reserve time for system pods if regular pods have adequate time to exit gracefully.
	if timeout >= 2*p.systemPodGracePeriod {
		gracePeriod = int64(timeout.Seconds() - p.systemPodGracePeriod.Seconds())
	}
	deleteOptions := &metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod}
	if err := p.deletePods(regularPods, deleteOptions); err != nil {
		return err
	}
	// Evict system pods.
	gracePeriod = int64(p.systemPodGracePeriod.Seconds())
	deleteOptions.GracePeriodSeconds = &gracePeriod
	if err := p.deletePods(systemPods, deleteOptions); err != nil {
		return err
	}
	glog.V(4).Infof("Successfully evicted all pods from node %q", p.node)
	return nil
}

func (p *podEvictionHandler) deletePods(pods []v1.Pod, deleteOptions *metav1.DeleteOptions) error {
	for _, pod := range pods {
		p.recorder.Eventf(&pod, v1.EventTypeWarning, eventReason, "Node %q is about to be terminated. Evicting pod prior to node termination.", p.node)
		// Delete the pod with the specified timeout.
		glog.V(4).Infof("About to delete pod %q in namespace %q", pod.Name, pod.Namespace)
		if err := p.client.Pods(pod.Namespace).Delete(pod.Name, deleteOptions); err != nil {
			glog.V(2).Infof("Failed to delete pod %q in namespace %q - %v", pod.Name, pod.Namespace, err)
			return err
		}
	}
	// wait for pods to be actually deleted since deletion is asynchronous & pods have a deletion grace period to exit gracefully.
	for _, pod := range pods {
		if err := p.waitForPodNotFound(pod.Name, pod.Namespace, time.Duration(*deleteOptions.GracePeriodSeconds)*time.Second); err != nil {
			glog.Errorf("Pod %q/%q did not get deleted within grace period %d seconds: %v", pod.Namespace, pod.Name, deleteOptions.GracePeriodSeconds, err)
		}
	}
	return nil
}

// waitForPodNotFound returns an error if it takes too long for the pod to fully terminate.
func (p *podEvictionHandler) waitForPodNotFound(podName, ns string, timeout time.Duration) error {
	return wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		_, err := p.client.Pods(ns).Get(podName, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return true, nil // done
		}
		if err != nil {
			return true, err // stop wait with error
		}
		return false, nil
	})
}
