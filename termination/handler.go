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
	"reflect"
	"syscall"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/golang/glog"
)

type nodeTerminationHandler struct {
	currentNodeState   NodeTerminationState
	taintHandler       NodeTaintHandler
	podEvictionHandler PodEvictionHandler
	terminationSource  NodeTerminationSource
	excludePods        map[string]string
}

func NewNodeTerminationHandler(
	source NodeTerminationSource,
	taintHandler NodeTaintHandler,
	evictionHandler PodEvictionHandler,
	excludePods map[string]string) NodeTerminationHandler {
	return &nodeTerminationHandler{
		taintHandler:       taintHandler,
		podEvictionHandler: evictionHandler,
		terminationSource:  source,
		excludePods:        excludePods,
	}
}

func (n *nodeTerminationHandler) processNodeState() error {
	// Handle regular node state.
	if !n.currentNodeState.PendingTermination {
		glog.V(4).Infof("No pending terminations. Removing taint")
		return n.taintHandler.RemoveTaint()
	}
	glog.V(4).Infof("Current node state: %v", n.currentNodeState)
	// Handle a node that is about to be terminated.
	// Log an event that a termination is impending.
	// Reserve some time for restarting the node.
	timeout := n.currentNodeState.TerminationTime.Sub(time.Now())
	// If the timeout is lesser than 2 minutes it is assumed that there isn't much time to reserve for restarts.
	// By default such nodes are preemptible nodes which do not benefit from node restarts.
	if timeout.Seconds() >= 120 {
		timeout = timeout - time.Minute
	}
	glog.V(4).Infof("Applying taint prior to handling termination")
	if err := n.taintHandler.ApplyTaint(); err != nil {
		return err
	}
	glog.V(4).Infof("Evicting all pods from the node")
	if err := n.podEvictionHandler.EvictPods(n.excludePods, timeout); err != nil {
		return err
	}
	if n.currentNodeState.NeedsReboot {
		glog.V(4).Infof("Rebooting the node")
		return n.rebootNode()
	}
	return nil
}

func (n *nodeTerminationHandler) rebootNode() error {
	// Sync the filesystem.
	syscall.Sync()
	// Reboot the node.
	return syscall.Reboot(syscall.LINUX_REBOOT_CMD_RESTART2)
}

func (n *nodeTerminationHandler) Start() error {
	n.currentNodeState = n.terminationSource.GetState()
	glog.V(4).Infof("Processing initial node state")
	if err := n.processNodeState(); err != nil {
		glog.V(2).Infof("Failed to process initial node state - %v", err)
		return err
	}
	for state := range n.terminationSource.WatchState() {
		if !reflect.DeepEqual(state, n.currentNodeState) {
			n.currentNodeState = state
			if err := wait.ExponentialBackoff(wait.Backoff{
				Duration: time.Second,
				Factor:   1.2,
				Steps:    10, // Results in ~6 seconds of wait at the max.
			}, func() (bool, error) {
				err := n.processNodeState()
				if err != nil {
					glog.Errorf("Failed to process node state update.\nState: %v\nError: %v", n.currentNodeState, err)
					return false, nil
				}
				return true, nil
			}); err != nil {
				return err
			}
		}
	}
	return nil
}
