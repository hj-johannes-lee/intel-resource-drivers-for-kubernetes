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

	"k8s.io/klog/v2"

	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/gohlml"
)

func monitorHealth(ctx context.Context) {
	ret := gohlml.InitWithLogs()
	if ret != nil {
		klog.Errorf("failed to initialize HLML: %v", ret)
		return
	}

	defer func() {
		ret := gohlml.Shutdown()
		if ret != nil {
			klog.Errorf("failed to shutdown HLML: %v", ret)
		}
	}()

	count, ret := gohlml.DeviceCount()
	if ret != nil {
		klog.Errorf("failed to get device count: %v", ret)
		return
	}

	for i := uint(0); i < count; i++ {
		device, ret := gohlml.DeviceHandleByIndex(i)
		if ret != nil {
			klog.Errorf("failed to get device at index %d: %v", i, ret)
			continue
		}

		uuid, ret := device.ModuleID()
		if ret != nil {
			klog.Errorf("failed to get uuid of device at index %d: %v", i, ret)
			continue
		}

		fmt.Printf("%v\n", uuid)
	}
}
