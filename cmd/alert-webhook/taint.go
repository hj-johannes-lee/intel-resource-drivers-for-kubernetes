/*
 * Copyright (c) 2023-2024, Intel Corporation.  All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	coreclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	intelclientset "github.com/intel/intel-resource-drivers-for-kubernetes/pkg/intel.com/resource/gpu/clientset/versioned"
	intelcrd "github.com/intel/intel-resource-drivers-for-kubernetes/pkg/intel.com/resource/gpu/v1alpha2/api"
)

type clientsetType struct {
	core  coreclientset.Interface
	intel intelclientset.Interface
}

// Tainter updates GpuAllocationState CR tainting section.
// Except for mutex, members are not updated after newTainter() call.
type tainter struct {
	ctx       context.Context
	nsname    string
	csconfig  *rest.Config
	clientset *clientsetType
	mutex     sync.Mutex
}

func newTainter(ctx context.Context, kubeconfig string) (*tainter, error) {
	klog.V(5).Info("newTainter()")

	csconfig, err := getClientsetConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("create client configuration: %v", err)
	}

	coreclient, err := coreclientset.NewForConfig(csconfig)
	if err != nil {
		return nil, fmt.Errorf("create core client: %v", err)
	}

	intelclient, err := intelclientset.NewForConfig(csconfig)
	if err != nil {
		return nil, fmt.Errorf("create Intel client: %v", err)
	}

	nsname, nsnamefound := os.LookupEnv("POD_NAMESPACE")
	if !nsnamefound {
		nsname = "default"
	}

	tainter := &tainter{
		ctx:      ctx,
		nsname:   nsname,
		csconfig: csconfig,
		clientset: &clientsetType{
			coreclient,
			intelclient,
		},
		mutex: sync.Mutex{},
	}

	klog.V(5).Infof("Tainter initialized: %+v", tainter)
	return tainter, nil
}

func getClientsetConfig(kubeconfig string) (*rest.Config, error) {
	klog.V(5).Info("getClientsetConfig()")

	kubeconfigEnv := os.Getenv("KUBECONFIG")
	if kubeconfigEnv != "" {
		klog.V(5).Info("Found KUBECONFIG environment variable set, using that...")
		kubeconfig = kubeconfigEnv
	}

	var csconfig *rest.Config
	var err error

	if kubeconfig == "" {
		csconfig, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("create in-cluster client configuration: %v", err)
		}
	} else {
		csconfig, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("create out-of-cluster client configuration: %v", err)
		}
	}

	return csconfig, nil
}

// convert string with comma separate items to list, with nil indicting "all" items,
// returns that and true to indicate success.
func string2list(value *string) ([]string, bool) {
	if value == nil || *value == "" {
		return nil, false
	}
	if *value == "all" {
		return nil, true
	}
	items := strings.Split(*value, ",")
	for _, name := range items {
		if strings.TrimSpace(name) == "" {
			return nil, false
		}
	}
	return items, true
}

// convert string with comma separate items to map, with nil indicting "all" items,
// returns that and true to indicate success.
func string2map(value *string) (map[string]bool, bool) {
	if value == nil || *value == "" {
		return nil, false
	}
	if *value == "all" {
		return nil, true
	}
	items := make(map[string]bool)
	for _, name := range strings.Split(*value, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, false
		}
		items[name] = true
	}
	return items, true
}

// Depending on CLI flags, list or update specified (or all) taint reasons for
// specified (or all) devices on specified (or all) nodes.
func (t *tainter) setTaintsFromFlags(f *cliFlags) error {
	klog.V(5).Info("setTaintsFromFlags()")
	if f.action == nil || *f.action == "" {
		klog.V(5).Info("No CLI action requested")
		return nil
	}
	action := *f.action
	if action != "list" && action != "taint" && action != "untaint" {
		return fmt.Errorf("invalid CLI action '%s'", action)
	}
	nodes, ok := string2list(f.nodes)
	if !ok {
		return fmt.Errorf("invalid nodes list for CLI action")
	}
	reasons, ok := string2list(f.reasons)
	if !ok {
		return fmt.Errorf("invalid taint reasons list for CLI action")
	}
	devices, ok := string2map(f.devices)
	if !ok {
		return fmt.Errorf("invalid devices list for CLI action")
	}

	// klog.Infof("action: %s=>%s, nodes: %s=>%+v, devices: %s=>%+v, reasons: %s=>%+v",
	//	   *f.action, action, *f.nodes, nodes, *f.devices, devices, *f.reasons, reasons)

	if action == "taint" && reasons == nil {
		return fmt.Errorf("no reasons specified for tainting")
	}
	// part of new functionality still missing
	if nodes == nil {
		return fmt.Errorf("TODO: support 'all' nodes")
	}

	for _, node := range nodes {
		if err := t.handleNodeAction(action, node, devices, reasons); err != nil {
			return err
		}
	}

	return nil
}

func (t *tainter) handleNodeAction(action, node string, devices map[string]bool, reasons []string) error {
	if action == "list" {
		return t.listNodeTaints(node)
	}

	// CRD access serialization
	t.mutex.Lock()
	defer t.mutex.Unlock()

	crdconfig := &intelcrd.GpuAllocationStateConfig{
		Namespace: t.nsname,
		Name:      node,
	}

	klog.V(5).Infof("New GAS for node '%s' in '%s' ns", node, t.nsname)
	gas := intelcrd.NewGpuAllocationState(crdconfig, t.clientset.intel)

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {

		var err error
		if err = gas.Get(t.ctx); err != nil {
			return err
		}

		var changed bool
		if action == "taint" {
			klog.V(3).Infof("Taint node '%s' GPUs with specified reasons", node)
			changed = addNodeTaints(&gas.Spec, node, devices, reasons)
		} else {
			klog.V(3).Infof("Remove specified taint reasons from node '%s' GPUs", node)
			changed = removeNodeTaints(&gas.Spec, node, devices, reasons)
		}

		if !changed {
			klog.V(3).Info("=> No changes needed")
			return nil
		}

		if err := gas.Update(t.ctx, &gas.Spec); err != nil {
			return err
		}
		return nil
	})
}

// add specified taint reasons for specified devices (nil=all) on node.
func addNodeTaints(spec *intelcrd.GpuAllocationStateSpec, node string, devices map[string]bool, reasons []string) bool {
	changed := false
	if spec.AllocatableDevices == nil {
		return changed
	}

	// Add given taint reason to specified GPUs on given node
	for uid := range spec.AllocatableDevices {
		if devices != nil && !devices[uid] {
			continue
		}
		// no need to taint VFs, PFs are enough
		if spec.AllocatableDevices[uid].Type == intelcrd.VfDeviceType {
			continue
		}

		for _, reason := range reasons {
			if addGpuTaint(spec, node, uid, reason) {
				changed = true
			}
		}
	}

	return changed
}

// remove specified taint reasons (nil=all) for specified devices (nil=all) on node.
func removeNodeTaints(spec *intelcrd.GpuAllocationStateSpec, node string, devices map[string]bool, reasons []string) bool {
	changed := false
	if spec.TaintedDevices == nil {
		return changed
	}

	for uid := range spec.TaintedDevices {
		if devices != nil && !devices[uid] {
			continue
		}
		if reasons == nil {
			if _, found := spec.TaintedDevices[uid]; found {
				// remove all reasons
				delete(spec.TaintedDevices, uid)
				changed = true
			}
			if len(spec.TaintedDevices) == 0 {
				spec.TaintedDevices = nil
				changed = true
			}
			continue
		}
		for _, reason := range reasons {
			if removeGpuTaint(spec, node, uid, reason) {
				changed = true
			}
		}
	}

	return changed
}

func (t *tainter) listNodeTaints(node string) error {
	klog.Infof("%s:", node)
	count := 0
	taints := t.getNodeTaints(node, time.Time{})
	for uid, reasons := range taints {
		i := 0
		names := make([]string, len(reasons))
		for reason := range reasons {
			names[i] = reason
			count++
			i++
		}
		klog.Infof("- %s: %v", uid, names)
	}
	if count == 0 {
		klog.Info("- NO tainted devices")
	}

	return nil
}

// getNodeTaints reads pre-existing taints from node's GAS CR, and returns
// gpu:reasonInfo taint map created from it, or nil if node did not have GPUs
func (t *tainter) getNodeTaints(node string, start time.Time) gpuTaints {
	klog.V(5).Info("getNodeTaints()")
	if node == "" {
		panic("getNodeTaints: no node or GPUs to update")
	}

	// CRD access serialization
	t.mutex.Lock()
	defer t.mutex.Unlock()

	crdconfig := &intelcrd.GpuAllocationStateConfig{
		Namespace: t.nsname,
		Name:      node,
	}

	klog.V(5).Infof("Get GAS for node '%s' in '%s' ns", node, t.nsname)
	gas := intelcrd.NewGpuAllocationState(crdconfig, t.clientset.intel)
	if err := gas.Get(t.ctx); err != nil {
		return nil
	}

	if gas.Spec.AllocatableDevices == nil {
		return nil
	}

	taints := make(gpuTaints)
	if gas.Spec.TaintedDevices == nil {
		return taints
	}

	// map string:bool to reasonInfo structs
	for uid, taint := range gas.Spec.TaintedDevices {
		taints[uid] = make(taintReasons)
		for name := range taint.Reasons {
			taints[uid][name] = reasonInfo{
				processed: true,
				status:    alertFiring,
				start:     start,
				name:      name,
			}
		}
	}

	klog.V(5).Infof("Pre-existing taints on node '%s': %+v", node, taints)
	return taints
}

// Update taint reasons for listed GPUs on given node.
func (t *tainter) updateNodeTaints(node string, taints gpuTaints) error {
	klog.V(5).Info("tainter.updateNodeTaints()")
	// TODO: add tests with notifications that miss these
	if node == "" || len(taints) == 0 {
		panic("updateNodeTaints: no node or GPUs to update")
	}

	// CRD access serialization
	t.mutex.Lock()
	defer t.mutex.Unlock()

	crdconfig := &intelcrd.GpuAllocationStateConfig{
		Namespace: t.nsname,
		Name:      node,
	}

	klog.V(5).Infof("New GAS for node '%s' in '%s' ns", node, t.nsname)
	gas := intelcrd.NewGpuAllocationState(crdconfig, t.clientset.intel)

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {

		if err := gas.Get(t.ctx); err != nil {
			return err
		}

		changed := false
		for uid, reasons := range taints {
			// no corresponding GPU?
			if _, found := gas.Spec.AllocatableDevices[uid]; !found {
				klog.V(3).Infof("Cannot update taints, '%s' node has no allocatable '%s' GPU", node, uid)
				continue
			}

			// VF may have changed as alerts are for longer period conditions
			// and VFs are transitory.  While taint could be mapped to PF if
			// some VF with a same UID still exits, that's not given. And lastly,
			// metrics for VFs are either same as for PF, or less relevant for
			// health, so it should be safe to ignore them.
			if gas.Spec.AllocatableDevices[uid].Type == intelcrd.VfDeviceType {
				klog.V(3).Infof("Ignoring alert notifications for node '%s' VF '%s'", node, uid)
				continue
			}

			for name, info := range reasons {
				var (
					updated bool
					verb    string
				)
				switch info.status {
				case alertFiring:
					updated = addGpuTaint(&gas.Spec, node, uid, name)
					verb = "added"
				case alertResolved:
					updated = removeGpuTaint(&gas.Spec, node, uid, name)
					verb = "removed"
				default:
					panic("unknown alert state")
				}

				if updated {
					klog.V(3).Infof("'%s' node '%s' GPU '%s' taint %s",
						node, uid, name, verb)
					if gas.Spec.TaintedDevices == nil {
						klog.V(3).Infof("Taints removed from all GPUs on node '%s'", node)
					}
					changed = true
				}
			}
		}

		if !changed {
			return nil
		}

		if err := gas.Update(t.ctx, &gas.Spec); err != nil {
			return err
		}

		return nil
	})
}

// addGpuTaint updates GAS spec by setting given taint reason for to given device
// in tainted devices map, or returns false if taint is already there.
func addGpuTaint(spec *intelcrd.GpuAllocationStateSpec, node, uid, reason string) bool {
	reasons := make(map[string]bool)
	reasons[reason] = true

	if spec.TaintedDevices == nil {
		spec.TaintedDevices = intelcrd.TaintedDevices{}
	}

	taint, found := spec.TaintedDevices[uid]
	if !found {
		spec.TaintedDevices[uid] = intelcrd.TaintedGpu{Reasons: reasons}
		return true
	}

	if taint.Reasons == nil || len(taint.Reasons) == 0 {
		klog.Warningf("node '%s' GPU '%s' has empty taint reasons map", node, uid)
		spec.TaintedDevices[uid] = intelcrd.TaintedGpu{Reasons: reasons}
		return true
	}

	if !taint.Reasons[reason] {
		spec.TaintedDevices[uid].Reasons[reason] = true
		return true
	}

	klog.V(5).Infof("Node '%s' GPU '%s' already tainted with '%s'", node, uid, reason)
	return false
}

// removeGpuTaint removes given taint reason from given GPU in GAS spec,
// or returns false if given taint reason was already missing.
func removeGpuTaint(spec *intelcrd.GpuAllocationStateSpec, node, uid, reason string) bool {
	if spec.TaintedDevices == nil {
		return false
	}

	taint, found := spec.TaintedDevices[uid]
	if !found {
		return false
	}

	if taint.Reasons == nil || len(taint.Reasons) == 0 {
		klog.Warningf("Removed empty taint map for node '%s' GPU '%s'", node, uid)
		delete(spec.TaintedDevices, uid)
		return true
	}

	if _, found = taint.Reasons[reason]; !found {
		// no match for resolved taint reason
		return false
	}

	if len(taint.Reasons) > 1 {
		delete(spec.TaintedDevices[uid].Reasons, reason)
		return true
	}

	// all taint reasons for given GPU gone
	delete(spec.TaintedDevices, uid)
	if len(spec.TaintedDevices) == 0 {
		spec.TaintedDevices = nil
	}
	return true
}
