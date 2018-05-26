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
	"testing"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
)

type pod struct {
	name, namespace, nodeName string
}

func makePod(p pod) v1.Pod {
	return v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.name,
			Namespace: p.namespace,
		},
		Spec: v1.PodSpec{
			NodeName: p.nodeName,
		},
	}
}

func TestEvictions(t *testing.T) {
	for _, test := range []struct {
		pods          []pod
		excludedPod   pod
		remainingPods []pod
	}{
		{
			pods: []pod{
				{
					name:      "foo",
					namespace: "default",
					nodeName:  "localhost",
				},
				{
					name:      "bar",
					namespace: "kube-system",
					nodeName:  "localhost",
				},
				{
					name:      "baz",
					namespace: "kube-system",
					nodeName:  "localhost",
				},
			},
			excludedPod: pod{
				name:      "baz",
				namespace: "kube-system",
			},
			remainingPods: []pod{
				{
					name:      "baz",
					namespace: "kube-system",
					nodeName:  "localhost",
				},
			},
		},
	} {
		var podList v1.PodList
		for _, p := range test.pods {
			podList.Items = append(podList.Items, makePod(p))
		}
		kubeClientset := fakekubeclientset.NewSimpleClientset(&podList)
		recorder := record.NewFakeRecorder(20)
		evictionHandler := &podEvictionHandler{
			client:               kubeClientset.CoreV1(),
			node:                 "localhost",
			recorder:             recorder,
			systemPodGracePeriod: 1,
		}
		excludePods := map[string]string{test.excludedPod.name: test.excludedPod.namespace}
		evictionHandler.EvictPods(excludePods, 30 /* timeout */)
		options := metav1.ListOptions{FieldSelector: fields.OneTermEqualSelector("spec.nodeName", string("localhost")).String()}
		pods, err := kubeClientset.CoreV1().Pods(metav1.NamespaceAll).List(options)
		if err != nil {
			t.Fatal(err)
		}

		if len(pods.Items) != len(test.remainingPods) {
			t.Fatalf("expected to see %d pods remaining, found %d remaining", len(test.remainingPods), len(pods.Items))
		}
	}
}
