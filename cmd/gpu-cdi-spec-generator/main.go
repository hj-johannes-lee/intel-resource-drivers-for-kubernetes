/*
 * Copyright (c) 2023, Intel Corporation.  All Rights Reserved.
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
	"fmt"
	"path/filepath"

	cdiapi "github.com/container-orchestrated-devices/container-device-interface/pkg/cdi"
	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/gpu/cdihelpers"
	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/gpu/device"
	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/gpu/discovery"
)

func main() {
	sysfsDir := device.GetSysfsDir()
	sysfsI915Dir := filepath.Join(sysfsDir, device.SysfsI915path)
	sysfsDRMDir := filepath.Join(sysfsDir, device.SysfsDRMpath)

	detectedDevices := discovery.DiscoverDevices(sysfsI915Dir, sysfsDRMDir)
	if len(detectedDevices) == 0 {
		fmt.Println("No supported devices detected")
	}

	fmt.Println("Getting CDI registry")
	cdi := cdiapi.GetRegistry(
		cdiapi.WithSpecDirs(device.CDIRoot),
	)

	fmt.Println("Got CDI registry, refreshing it")
	err := cdi.Refresh()
	if err != nil {
		fmt.Printf("unable to refresh the CDI registry: %v", err)
		return
	}

	// syncDetectedDevicesWithCdiRegistry overrides uid in detecteddevices from existing cdi spec
	err = cdihelpers.SyncDetectedDevicesWithCdiRegistry(cdi, detectedDevices, true)
	if err != nil {
		fmt.Printf("unable to sync detected devices to CDI registry: %v", err)
	}

}
