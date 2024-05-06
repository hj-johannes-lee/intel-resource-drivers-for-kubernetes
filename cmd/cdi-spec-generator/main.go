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
	"fmt"
	"os"
	"strings"

	cdiapi "github.com/container-orchestrated-devices/container-device-interface/pkg/cdi"
	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/gpu/cdihelpers"
	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/gpu/device"
	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/gpu/discovery"
	"github.com/spf13/cobra"
	cliflag "k8s.io/component-base/cli/flag"
)

// Flags holds input parameter flags.
type flagsType struct {
	deviceType *string
}

func main() {
	command := newCommand()
	err := command.Execute()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func newCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "CDI Spec Generator",
		Short: "Intel CDI Spec Generator",
	}

	flags := addFlags(cmd)

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		err := validateFlags(flags)
		if err != nil {
			return fmt.Errorf("failed parsing parameters: %v", err)
		}

		switch strings.ToLower(*flags.deviceType) {
		case "gpu":
			return getGPUDevice()
		case "gaudi":
			return getGaudiDevice()
		default:
			return fmt.Errorf("invalid device type specified: %s", *flags.deviceType)
		}
	}

	return cmd
}

func addFlags(cmd *cobra.Command) *flagsType {
	flags := &flagsType{}
	sharedFlagSets := cliflag.NamedFlagSets{}

	fs := sharedFlagSets.FlagSet("CDI spec generator devices")
	flags.deviceType = fs.String("device type", "", "specify the type of device (e.g., 'gpu', 'gaudi')")

	fs = cmd.PersistentFlags()
	for _, f := range sharedFlagSets.FlagSets {
		fs.AddFlagSet(f)
	}
	return flags
}

func validateFlags(f *flagsType) error {
	if *f.deviceType == "" {
		return fmt.Errorf("device type must be specified")
	}

	switch strings.ToLower(*f.deviceType) {
	case "gpu", "gaudi":
		return nil
	default:
		return fmt.Errorf("unknown device type: %v", *f.deviceType)
	}
}

func getGPUDevice() error {
	sysfsDir := device.GetSysfsDir()

	detectedDevices := discovery.DiscoverDevices(sysfsDir)
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
		return err
	}

	// syncDetectedDevicesWithCdiRegistry overrides uid in detecteddevices from existing cdi spec
	err = cdihelpers.SyncDetectedDevicesWithRegistry(cdi, detectedDevices, true)
	if err != nil {
		fmt.Printf("unable to sync detected devices to CDI registry: %v", err)
	}

	return nil
}

func getGaudiDevice() error {
	return fmt.Errorf("not implemented")
}
