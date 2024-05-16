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
	"maps"
	"os"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// convert string with comma separated items to list, with nil indicating "all" items,
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

// convert string with comma separated items to map, with nil indicating "all" items,
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

// if nodes names are specified, return those as a list, otherwise fetch & return list
// of names for all cluster nodes, and true to indicate success.
func (t *tainter) expandNodes(value *string) ([]string, bool) {
	nodes, ok := string2list(value)
	if !ok || nodes != nil {
		return nodes, ok
	}

	items, err := t.clientset.core.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Errorf("Listing cluster nodes failed: %v", err)
		return nodes, false
	}

	i := 0
	nodes = make([]string, len(items.Items))
	for _, node := range items.Items {
		nodes[i] = node.Name
		i++
	}

	return nodes, true
}

type taintInfoType struct {
	reasons map[string]bool
	devices int
	tainted int
}

// 'nil' value = all items (both for devices & reasons).
type taintArgsType struct {
	devices map[string]bool
	reasons []string
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

	nodes, ok := t.expandNodes(f.nodes)
	if !ok {
		return fmt.Errorf("node list '%v' creation failed for CLI action", f.nodes)
	}

	args := taintArgsType{}
	if args.reasons, ok = string2list(f.reasons); !ok {
		return fmt.Errorf("invalid taint reasons list '%v' for CLI action", f.reasons)
	}

	if args.devices, ok = string2map(f.devices); !ok {
		return fmt.Errorf("invalid devices list '%v' for CLI action", f.devices)
	}

	if action == "taint" && args.reasons == nil {
		return fmt.Errorf("no reasons specified for tainting")
	}

	info := taintInfoType{
		reasons: make(map[string]bool),
	}

	for _, node := range nodes {
		if err := t.handleNodeAction(&info, action, node, args); err != nil {
			return err
		}
	}

	taintInfoSummary(info, len(nodes), action)

	return nil
}

func taintInfoSummary(info taintInfoType, nodeCount int, action string) {
	if action != "list" {
		// TODO: collect & output summary info also for other actions?
		return
	}

	klog.Info("Summary:")

	if info.devices == 0 {
		klog.Infof("- No (matching) devices on specified %d nodes", nodeCount)
		return
	}
	klog.Infof("- %d devices on %d nodes", info.devices, nodeCount)

	if info.tainted == 0 {
		if len(info.reasons) > 0 {
			panic("taint reasons not empty, although tainted dev count = 0")
		}
		klog.Info("- None tainted (matching specified devices/reasons)")
		return
	}

	if len(info.reasons) == 0 {
		panic("taint reasons is empty, although tainted dev count != 0")
	}

	klog.Infof("- %d of them tainted", info.tainted)

	klog.Info("Unique taint reasons:")
	for name := range info.reasons {
		klog.Infof("- %s", name)
	}
}

func (t *tainter) handleNodeAction(info *taintInfoType, action, node string, args taintArgsType) error {
	// CRD access serialization
	t.mutex.Lock()
	defer t.mutex.Unlock()

	crdconfig := &intelcrd.GpuAllocationStateConfig{
		Namespace: t.nsname,
		Name:      node,
	}

	klog.V(5).Infof("New '%s' action for '%s' node in '%s' ns", action, node, t.nsname)
	gas := intelcrd.NewGpuAllocationState(crdconfig, t.clientset.intel)

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {

		if err := gas.Get(t.ctx); err != nil {
			klog.V(3).Infof("%s:", node)
			klog.V(3).Info("- NO device information (node or its GAS CR missing)")
			return nil
		}

		var changed bool
		switch action {
		case "list":
			return t.listNodeTaints(info, &gas.Spec, node, args)
		case "taint":
			klog.V(3).Infof("Taint node '%s' GPUs with specified reasons", node)
			changed = addNodeTaints(&gas.Spec, node, args)
		case "untaint":
			klog.V(3).Infof("Remove specified taint reasons from node '%s' GPUs", node)
			changed = removeNodeTaints(&gas.Spec, node, args)
		default:
			panic(fmt.Sprintf("unknown action %v", action)) // bug in caller
		}

		if !changed {
			klog.V(3).Info("=> No changes needed")
			return nil
		}

		return gas.Update(t.ctx, &gas.Spec)
	})
}

// add specified taint reasons for specified devices (nil=all) on node.
func addNodeTaints(spec *intelcrd.GpuAllocationStateSpec, node string, args taintArgsType) bool {
	changed := false
	if spec.AllocatableDevices == nil {
		return changed
	}

	// Add given taint reason to specified GPUs on given node
	for uid := range spec.AllocatableDevices {
		if args.devices != nil && !args.devices[uid] {
			continue
		}
		// no need to taint VFs, PFs are enough
		if spec.AllocatableDevices[uid].Type == intelcrd.VfDeviceType {
			continue
		}

		for _, reason := range args.reasons {
			if addGpuTaint(spec, node, uid, reason) {
				changed = true
			}
		}
	}

	return changed
}

// remove specified taint reasons (nil=all) for specified devices (nil=all) on node.
func removeNodeTaints(spec *intelcrd.GpuAllocationStateSpec, node string, args taintArgsType) bool {
	changed := false
	if spec.TaintedDevices == nil {
		return changed
	}

	for uid := range spec.TaintedDevices {
		if args.devices != nil && !args.devices[uid] {
			continue
		}

		if args.reasons == nil {
			// remove all reasons
			delete(spec.TaintedDevices, uid)

			if len(spec.TaintedDevices) == 0 {
				spec.TaintedDevices = nil
			}
			changed = true
			continue
		}

		for _, reason := range args.reasons {
			if removeGpuTaint(spec, node, uid, reason) {
				changed = true
			}
		}
	}

	return changed
}

// list all available devices and their taint reasons on given node, filtered by
// given devices + reasons lists. Output warnings on invalid taint information.
func (t *tainter) listNodeTaints(info *taintInfoType, spec *intelcrd.GpuAllocationStateSpec, node string, args taintArgsType) error {
	klog.Infof("%s:", node)

	checkTaints(spec)

	if spec.AllocatableDevices == nil {
		klog.Info("- NO devices")
		return nil
	}

	total := 0
	tainted := 0
	unique := make(map[string]bool)

	for uid := range spec.AllocatableDevices {
		if args.devices != nil && !args.devices[uid] {
			continue
		}
		total++

		if spec.TaintedDevices == nil {
			klog.Infof("- %s", uid)
			continue
		}

		taint, found := spec.TaintedDevices[uid]
		if !found || len(taint.Reasons) == 0 {
			klog.Infof("- %s", uid)
			if found {
				klog.Infof("  - WARN: empty (instead of missing) taint reasons", node)
			}
			continue
		}

		names := make([]string, 0)

		if args.reasons != nil {
			// filtered list of reasons
			for _, name := range args.reasons {
				if _, found := taint.Reasons[name]; found {
					names = append(names, name)
					unique[name] = true
				}
			}
		} else {
			// all reasons
			for name := range taint.Reasons {
				names = append(names, name)
				unique[name] = true
			}
		}

		if len(names) > 0 {
			tainted++
		}

		klog.Infof("- %s: %v", uid, names)
	}

	if len(unique) > 0 {
		maps.Copy(info.reasons, unique)
	}
	info.tainted += tainted
	info.devices += total

	return nil
}

// check taint info against available device info and warn of mismatches.
func checkTaints(spec *intelcrd.GpuAllocationStateSpec) {
	if spec.TaintedDevices == nil {
		return
	}

	if len(spec.TaintedDevices) == 0 {
		klog.Info("- WARN: empty (instead of missing) tainted devices list")
		return
	}

	if spec.AllocatableDevices == nil {
		klog.Infof("- WARN: %d tainted devices, although no available devices",
			len(spec.TaintedDevices))
		return
	}

	for uid := range spec.TaintedDevices {
		if _, found := spec.AllocatableDevices[uid]; found {
			continue
		}
		klog.Infof("- WARN: '%s' device listed as tainted, does not exist!", uid)
	}
}

// getNodeTaints reads pre-existing taints from node's GAS CR, and returns
// gpu:reasonInfo taint map created from it, or nil if node did not have GPUs
func (t *tainter) getNodeTaints(node string, start time.Time) gpuTaints {
	klog.V(5).Info("getNodeTaints()")
	if node == "" {
		panic("getNodeTaints: no node specified")
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
