/*
 * Copyright (c) 2024, Intel Corporation.  All Rights Reserved.
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
	"path"

	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	drav1alpha2 "k8s.io/kubelet/pkg/apis/dra/v1alpha2"
	drav1alpha3 "k8s.io/kubelet/pkg/apis/dra/v1alpha3"

	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/gaudi/device"
	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/gaudi/discovery"
	intelcrd "github.com/intel/intel-resource-drivers-for-kubernetes/pkg/intel.com/resource/gaudi/v1alpha1/api"
	driverVersion "github.com/intel/intel-resource-drivers-for-kubernetes/pkg/version"
)

// compile-time test for implementation conformance with the interface.
var _ drav1alpha3.NodeServer = (*driver)(nil) // K8s v1.28 ~ v1.30.
var _ drav1alpha2.NodeServer = (*driver)(nil) // K8s v1.27 ~ v1.30.

type driver struct {
	// Resource model publisher uses this channel to know when to send updated model.
	updateCh chan bool
	// Resource model publisher uses this channel to know when to stop sending updates to the kubelet and quit.
	doneCh   chan bool
	gas      *intelcrd.GaudiAllocationState
	state    *nodeState
	sysfsDir string
}

func newDriver(ctx context.Context, config *configType) (*driver, error) {
	var state *nodeState

	driverVersion.PrintDriverVersion(intelcrd.APIGroupName, intelcrd.APIVersion)

	sysfsDir := device.GetSysfsRoot()
	gas := intelcrd.NewGaudiAllocationState(config.crdconfig, config.clientset.intel)

	preparedClaimsFilePath := path.Join(config.driverPluginPath, device.PreparedClaimsFileName)

	setupErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		klog.V(3).Info("Creating new GaudiAllocationState")
		err := gas.GetOrCreate(ctx)
		if err != nil {
			return fmt.Errorf("failed to get GaudiAllocationState: %v", err)
		}

		klog.V(3).Info("Setting GaudiAllocationState as NotReady")
		err = gas.UpdateStatus(ctx, intelcrd.GaudiAllocationStateStatusNotReady)
		if err != nil {
			return fmt.Errorf("failed to set GaudiAllocationState as NotReady: %v", err)
		}

		detectedDevices := discovery.DiscoverDevices(sysfsDir, device.DefaultNamingStyle)
		if len(detectedDevices) == 0 {
			klog.Info("No supported devices detected")
		}

		klog.V(3).Info("Creating new NodeState")
		state, err = newNodeState(gas, detectedDevices, config.cdiRoot, preparedClaimsFilePath)
		if err != nil {
			return fmt.Errorf("failed to create new NodeState: %v", err)
		}

		klog.V(3).Info("Updating GaudiAllocationState with detected devices")
		err = gas.Update(ctx, state.GetUpdatedSpec(&gas.Spec))
		if err != nil {
			return fmt.Errorf("failed to update GaudiAllocationState: %v", err)
		}

		klog.V(3).Info("Setting GaudiAllocationState status as Ready")
		return gas.UpdateStatus(ctx, intelcrd.GaudiAllocationStateStatusReady)
	})
	if setupErr != nil {
		return nil, fmt.Errorf("creating driver: %v", setupErr)
	}

	d := &driver{
		gas:      gas,
		state:    state,
		sysfsDir: sysfsDir,
	}
	klog.V(3).Info("Finished creating new driver")

	return d, nil
}

// NodePrepareResource provides backwards compatibility with K8s v1.27 that has only DRA API v1alpha2 in kubelet.
func (d *driver) NodePrepareResource(ctx context.Context, req *drav1alpha2.NodePrepareResourceRequest) (*drav1alpha2.NodePrepareResourceResponse, error) {
	klog.FromContext(ctx).V(5).Info("NodePrepareResourceCalled", req)
	claim := &drav1alpha3.Claim{}
	claim.Namespace = req.Namespace
	claim.Uid = req.ClaimUid
	claim.Name = req.ClaimName
	claim.ResourceHandle = req.ResourceHandle

	v1alpha3Response := d.nodePrepareResource(ctx, claim)

	if v1alpha3Response.Error != "" {
		return nil, fmt.Errorf(v1alpha3Response.Error)
	}

	response := &drav1alpha2.NodePrepareResourceResponse{}
	response.CdiDevices = v1alpha3Response.CDIDevices

	return response, nil
}

func (d *driver) NodePrepareResources(ctx context.Context, req *drav1alpha3.NodePrepareResourcesRequest) (*drav1alpha3.NodePrepareResourcesResponse, error) {
	klog.V(5).Infof("NodePrepareResource is called: request: %+v", req)

	preparedResources := &drav1alpha3.NodePrepareResourcesResponse{Claims: map[string]*drav1alpha3.NodePrepareResourceResponse{}}

	for _, claim := range req.Claims {
		if claim.StructuredResourceHandle != nil && len(claim.StructuredResourceHandle) != 0 {
			preparedResources.Claims[claim.Uid] = d.nodePrepareStructuredResource(claim)
		} else {
			preparedResources.Claims[claim.Uid] = d.nodePrepareResource(ctx, claim)
		}
	}

	return preparedResources, nil
}

func (d *driver) nodePrepareResource(ctx context.Context, claim *drav1alpha3.Claim) *drav1alpha3.NodePrepareResourceResponse {
	klog.V(5).Infof("nodePrepareResource is called: request: %+v", claim)

	var cdinames []string

	// provide all devices for monitoring claims
	if claim.ResourceHandle == intelcrd.MonitorAllocType {
		cdinames = d.state.getMonitorCDINames(claim.Uid)
		klog.V(3).Infof("Prepared devices for monitor claim '%v': %s", claim.Uid, cdinames)
		return &drav1alpha3.NodePrepareResourceResponse{CDIDevices: cdinames}
	}

	if _, found := d.state.prepared[claim.Uid]; found {
		klog.V(3).Infof("Claim %s was already prepared, nothing to do", claim.Uid)
		return d.cdiDevices(claim.Uid)
	}

	err := d.gas.Get(ctx)
	if err != nil {
		return &drav1alpha3.NodePrepareResourceResponse{Error: fmt.Sprintf("failed to get GaudiAllocationState: %v", err)}
	}

	claimDevices, err := d.sanitizeClaimDevices(claim.Uid)
	if err != nil {
		return &drav1alpha3.NodePrepareResourceResponse{Error: fmt.Sprintf("failed validating devices to prepare: %v", err)}
	}

	// add resource claim to prepared list
	err = d.state.makePreparedClaimAllocation(claim.Uid, claimDevices)
	if err != nil {
		return &drav1alpha3.NodePrepareResourceResponse{Error: fmt.Sprintf("failed creating prepared claim allocation: %v", err)}
	}

	return d.cdiDevices(claim.Uid)
}

func (d *driver) cdiDevices(claimUID string) *drav1alpha3.NodePrepareResourceResponse {

	cdinames := d.state.GetAllocatedCDINames(claimUID)
	if len(cdinames) == 0 {
		klog.Errorf("could not find CDI device name from CDI registry for claim %s", claimUID)
		return &drav1alpha3.NodePrepareResourceResponse{Error: "error preparing resource: CDI devices not found in specs"}
	}

	klog.V(3).Infof("Prepared devices for claim '%v': %s", claimUID, cdinames)
	return &drav1alpha3.NodePrepareResourceResponse{CDIDevices: cdinames}
}

// NodeUnprepareResource provides backwards compatibility with K8s v1.27 that has only DRA API v1alpha2 in kubelet.
func (d *driver) NodeUnprepareResource(ctx context.Context, req *drav1alpha2.NodeUnprepareResourceRequest) (*drav1alpha2.NodeUnprepareResourceResponse, error) {
	claim := &drav1alpha3.Claim{}
	claim.Namespace = req.Namespace
	claim.Uid = req.ClaimUid
	claim.Name = req.ClaimName
	claim.ResourceHandle = req.ResourceHandle

	v1alpha3Response := d.nodeUnprepareResource(ctx, claim)

	if v1alpha3Response.Error != "" {
		return nil, fmt.Errorf(v1alpha3Response.Error)
	}

	return &drav1alpha2.NodeUnprepareResourceResponse{}, nil
}

func (d *driver) NodeUnprepareResources(ctx context.Context, req *drav1alpha3.NodeUnprepareResourcesRequest) (*drav1alpha3.NodeUnprepareResourcesResponse, error) {
	klog.V(5).Infof("NodeUnprepareResource is called: number of claims: %d", len(req.Claims))
	unpreparedResources := &drav1alpha3.NodeUnprepareResourcesResponse{
		Claims: map[string]*drav1alpha3.NodeUnprepareResourceResponse{},
	}

	for _, claim := range req.Claims {
		unpreparedResources.Claims[claim.Uid] = d.nodeUnprepareResource(ctx, claim)
	}

	return unpreparedResources, nil
}

func (d *driver) nodeUnprepareResource(ctx context.Context, claim *drav1alpha3.Claim) *drav1alpha3.NodeUnprepareResourceResponse {
	klog.V(3).Infof("NodeUnprepareResource is called: claim: %+v", claim)

	// no-op for monitoring claims
	if claim.ResourceHandle == intelcrd.MonitorAllocType {
		klog.V(3).Infof("Freed devices for monitor claim '%v'", claim.Uid)
		return &drav1alpha3.NodeUnprepareResourceResponse{}
	}

	err := d.state.FreeClaimDevices(claim.Uid)
	if err != nil {
		return &drav1alpha3.NodeUnprepareResourceResponse{Error: fmt.Sprintf("error freeing devices: %v", err)}
	}

	klog.V(3).Infof("Freed devices for claim '%v'", claim.Uid)
	return &drav1alpha3.NodeUnprepareResourceResponse{}
}

// sanitizeClaimDevices returns a slice of allocated devices after sanitizing or an error
// in case sanitization failed.
func (d *driver) sanitizeClaimDevices(claimUID string) ([]*device.DeviceInfo, error) {
	claimDevices := []*device.DeviceInfo{}

	claimAllocation, found := d.gas.Spec.AllocatedClaims[claimUID]
	if !found {
		return nil, fmt.Errorf("no allocation found for claim %v in API", claimUID)
	}

	for _, gaudi := range claimAllocation.Devices {
		if _, found := d.gas.Spec.AllocatableDevices[gaudi.UID]; !found {
			return nil, fmt.Errorf("allocated device %v not found in API", gaudi.UID)
		}

		if _, found := d.state.allocatable[gaudi.UID]; !found {
			return nil, fmt.Errorf("allocated device %v not found locally", gaudi.UID)
		}

		if _, tainted := d.gas.Spec.TaintedDevices[gaudi.UID]; tainted {
			return nil, fmt.Errorf("allocated device %v is tainted so it cannot be used", gaudi.UID)
		}

		claimDevices = append(claimDevices, d.state.allocatable[gaudi.UID])
	}

	return claimDevices, nil
}
