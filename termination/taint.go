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
	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	client "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/apis/core/helper"
)

type nodeTaintHandler struct {
	taint      v1.Taint
	annotation string
	node       string
	client     *client.Clientset
	recorder   record.EventRecorder
}

const (
	taintReason   = "ImpendingNodeTermination"
	untaintReason = "NoImpendingNodeTermination"
)

func NewNodeTaintHandler(taint v1.Taint, annotation, node string, client *client.Clientset, recorder record.EventRecorder) NodeTaintHandler {
	return &nodeTaintHandler{
		taint:      taint,
		annotation: annotation,
		node:       node,
		client:     client,
		recorder:   recorder,
	}
}

func (n *nodeTaintHandler) ApplyTaint() error {
	var (
		node    *v1.Node
		err     error
		updated bool
	)

	node, err = n.client.CoreV1().Nodes().Get(n.node, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if n.annotation != "" {
		node.Annotations[n.annotation] = "true"
		updated = true
	} else {
		node, updated = addOrUpdateTaint(node, &n.taint)
		glog.V(4).Infof("Node %q taints after removal; updated %v: %v", n.node, updated, node.Spec.Taints)
	}
	if updated {
		if _, err = n.client.CoreV1().Nodes().Update(node); err != nil {
			glog.V(2).Infof("Failed to update node object: %v", err)
			return err
		}
		n.recorder.Event(node, v1.EventTypeWarning, taintReason, "Node about to be terminated. Tainting the node to prevent further pods from being scheduling on the node")
	}
	return nil
}

func (n *nodeTaintHandler) RemoveTaint() error {
	node, err := n.client.CoreV1().Nodes().Get(n.node, metav1.GetOptions{})
	if err != nil {
		glog.V(2).Infof("Failed to remove taint: %v", err)
		return err
	}
	var updated bool
	if n.annotation != "" {
		node.Annotations[n.annotation] = "false"
		updated = true
	} else {
		node, updated = removeTaint(node, &n.taint)
	}
	if updated {
		if _, err = n.client.CoreV1().Nodes().Update(node); err != nil {
			return err
		}
		// Log an event that a termination is impending.
		n.recorder.Eventf(node, v1.EventTypeNormal, untaintReason, "Removing impending termination taint")
	}
	return nil
}

// AddOrUpdateTaint tries to add a taint to taint list. Returns a new copy of updated Node and true if something was updated
// false otherwise.
func addOrUpdateTaint(node *v1.Node, taint *v1.Taint) (*v1.Node, bool) {
	newNode := node.DeepCopy()
	nodeTaints := newNode.Spec.Taints

	var newTaints []v1.Taint
	updated := false
	for i := range nodeTaints {
		if taint.MatchTaint(&nodeTaints[i]) {
			if helper.Semantic.DeepEqual(*taint, nodeTaints[i]) {
				return newNode, false
			}
			newTaints = append(newTaints, *taint)
			updated = true
			continue
		}

		newTaints = append(newTaints, nodeTaints[i])
	}

	if !updated {
		newTaints = append(newTaints, *taint)
	}

	newNode.Spec.Taints = newTaints
	return newNode, true
}

func removeTaint(node *v1.Node, taint *v1.Taint) (*v1.Node, bool) {
	newNode := node.DeepCopy()
	nodeTaints := newNode.Spec.Taints
	if len(nodeTaints) == 0 {
		return newNode, false
	}

	if !taintExists(nodeTaints, taint) {
		return newNode, false
	}

	newTaints, _ := deleteTaint(nodeTaints, taint)
	newNode.Spec.Taints = newTaints
	return newNode, true
}

func taintExists(taints []v1.Taint, taintToFind *v1.Taint) bool {
	for _, taint := range taints {
		if taint.MatchTaint(taintToFind) {
			return true
		}
	}
	return false
}

func deleteTaint(taints []v1.Taint, taintToDelete *v1.Taint) ([]v1.Taint, bool) {
	newTaints := []v1.Taint{}
	deleted := false
	for i := range taints {
		if taintToDelete.MatchTaint(&taints[i]) {
			deleted = true
			continue
		}
		newTaints = append(newTaints, taints[i])
	}
	return newTaints, deleted
}
