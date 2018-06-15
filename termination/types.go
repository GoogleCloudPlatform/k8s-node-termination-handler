// Copyright 2018 Google Inc. All Rights Reserved.
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

import "time"

// NodeTerminationState represents the current status of a node in terms of terminations.
type NodeTerminationState struct {
	// NodeName is the hostname of the local node
	NodeName string
	// Set to true when a termination is impending for this node.
	PendingTermination bool
	// Aboslute time at which the node is expected to be terminated.
	TerminationTime time.Time
	// NeedsReboot indicates if a reboot is applicable to handle the pending termination.
	NeedsReboot bool
}

// NodeTerminationSource is an abstract repsentation of objects that tracks impending terminations for a node.
type NodeTerminationSource interface {
	// WatchStart launches an internal goroutine that will watch for VM terminations and publish updates via an output channel
	WatchState() <-chan NodeTerminationState
	// GetState returns the current state of a node in terms of pending terminations.
	GetState() NodeTerminationState
}

// NodeTaintHandler is an abstract representation of objects that can taint or untaint a k8s node.
type NodeTaintHandler interface {
	// ApplyTaint taints the node with a special taint specified during object initialization.
	ApplyTaint() error
	// RemoveTaint untaints the node of a special taint specified during object initialization.
	RemoveTaint() error
}

// PodEvictionHandler is an abstract representation of objects that can delete pods from all namespaces running on a specified node.
type PodEvictionHandler interface {
	// EvictPods deletes all pods except the ones included in `excludePods`
	// `excludePods` is a map where the key is pod name and value is pod namespace.
	// `timeout` is the overall time available to evict all pods.
	EvictPods(excludePods map[string]string, timeout time.Duration) error
}

// NodeTerminationHandler is an abstract representation of objects that can handle node terminations gracefully.
type NodeTerminationHandler interface {
	// Start runs the termination handler synchronously and returns error upon failure.
	Start() error
}
