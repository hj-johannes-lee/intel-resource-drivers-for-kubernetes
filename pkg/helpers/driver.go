/*
 * Copyright (c) 2025, Intel Corporation.  All Rights Reserved.
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

package helpers

import (
	"context"
	"fmt"

	"k8s.io/klog/v2"
	drav1 "k8s.io/kubelet/pkg/apis/dra/v1beta1"

	coreclientset "k8s.io/client-go/kubernetes"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"
)

type Driver struct {
	Client coreclientset.Interface
	State  *NodeState
	Plugin kubeletplugin.DRAPlugin
}

func (d *Driver) NodeUnprepareResources(ctx context.Context, req *drav1.NodeUnprepareResourcesRequest) (*drav1.NodeUnprepareResourcesResponse, error) {
	klog.V(5).Infof("NodeUnprepareResource is called: number of claims: %d", len(req.Claims))
	unpreparedResources := &drav1.NodeUnprepareResourcesResponse{
		Claims: map[string]*drav1.NodeUnprepareResourceResponse{},
	}

	for _, claim := range req.Claims {
		result := &drav1.NodeUnprepareResourceResponse{}
		if err := d.State.Unprepare(ctx, claim.UID); err != nil {
			result.Error = fmt.Sprintf("could not unprepare resource: %v", err)
		}

		unpreparedResources.Claims[claim.UID] = result
	}

	return unpreparedResources, nil
}

func (d *Driver) Shutdown(ctx context.Context) error {
	d.Plugin.Stop()
	return nil
}
