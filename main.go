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

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/k8s-node-termination-handler/termination"
	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
)

const eventSource = "NodeTerminationHandler"

var (
	inClusterVar        = flag.Bool("in-cluster", true, "Set to false if run outside of a k8s cluster.")
	regularVMTimeoutVar = flag.Duration("regular-vm-timeout", time.Hour, "Termination timeout for regular VMs. Defaults to an hour which is the timeout duration of GPU VMs.")
	excludePodsVar      = flag.String("exclude-pods", "", "List of pods to exclude from graceful eviction. Expected format is comma separated 'podName:podNamespace'.")
	kubeconfig          *string
	// TODO: Update this to use NoExecute taints once that graduates out of alpha.
	taintVar                = flag.String("taint", "", "Taint to place on the node while handling terminations. Example: cloud.google.com/impending-node-termination::NoSchedule")
	annotationVar           = flag.String("annotation", "", "Annotation to set on Node objects while handling terminations")
	systemPodGracePeriodVar = flag.Duration("system-pod-grace-period", 30*time.Second, "Time required for system pods to exit gracefully.")
)

func main() {
	if home := os.Getenv("HOME"); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()
	client, err := getKubeClient()
	if err != nil {
		glog.Fatalf("Failed to get kubernetes API Server Client. Error: %v", err)
	}
	excludePods, err := processExcludePods()
	if err != nil {
		glog.Fatal(err)
	}
	if *taintVar == "" && *annotationVar == "" {
		glog.Fatalf("Must specify one of taint or annotation")
	}
	taint, err := processTaint()
	if err != nil {
		glog.Fatal(err)
	}
	glog.Infof("Excluding pods %v", excludePods)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: client.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: eventSource})
	gceTerminationSource, err := termination.NewGCETerminationSource(*regularVMTimeoutVar)
	if err != nil {
		glog.Fatal(err)
	}
	nodeName := gceTerminationSource.GetState().NodeName
	taintHandler := termination.NewNodeTaintHandler(taint, *annotationVar, nodeName, client, recorder)
	evictionHandler := termination.NewPodEvictionHandler(nodeName, client, recorder, *systemPodGracePeriodVar)
	terminationHandler := termination.NewNodeTerminationHandler(gceTerminationSource, taintHandler, evictionHandler, excludePods)
	err = terminationHandler.Start()
	if err != nil {
		glog.Fatal(err)
	}
	glog.Fatalf("Unexpected execution flow")
}

func getKubeClient() (*kubernetes.Clientset, error) {
	var (
		config *rest.Config
		err    error
	)
	if *inClusterVar {
		// creates the in-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			return nil, err
		}
	}
	glog.V(10).Infof("Using kube config: %+v", config)
	// creates the clientset
	return kubernetes.NewForConfig(config)
}

func processExcludePods() (map[string]string, error) {
	ret := map[string]string{}
	for _, p := range strings.Split(*excludePodsVar, ",") {
		parts := strings.Split(p, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("Invalid value specified for --exclude-pods flag - %v", *excludePodsVar)
		}
		ret[parts[0]] = parts[1]
	}
	return ret, nil
}

func processTaint() (*v1.Taint, error) {
	if len(*annotationVar) != 0 && len(*taintVar) != 0 {
		return nil, fmt.Errorf("Annotation must not be specified when taints are specified")
	}
	if len(*taintVar) == 0 {
		return nil, nil
	}
	parts := strings.Split(*taintVar, ":")
	if len(parts) != 3 {
		return nil, fmt.Errorf("Invalid value specified for --taint flag. Expected format 'name:value:effect'. Input is %q", *taintVar)
	}
	return &v1.Taint{
		Key:    parts[0],
		Value:  parts[1],
		Effect: v1.TaintEffect(parts[2]),
	}, nil
}
