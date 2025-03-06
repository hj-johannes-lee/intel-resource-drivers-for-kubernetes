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
	"time"

	hlml "github.com/HabanaAI/gohlml"
	"k8s.io/klog/v2"

	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/gaudi/device"
)

const (
	healthCheckIntervalSeconds = int(10)
)

// initHLML loops through devices HLML detecs to update serial number in allocatable.
// This is needed for health monitoring, critical events contain device serial ID.
func (d *driver) initHLML(ctx context.Context) error {
	ret := hlml.InitWithLogs()
	if ret != nil {
		return fmt.Errorf("failed to initialize HLML: %v", ret)
	}

	count, ret := hlml.DeviceCount()
	if ret != nil {
		return fmt.Errorf("failed to get device count: %v", ret)
	}

	for i := uint(0); i < count; i++ {
		hlmlDevice, ret := hlml.DeviceHandleByIndex(i)
		if ret != nil {
			return fmt.Errorf("failed to get device at index %d: %v", i, ret)
		}

		serial, err := hlmlDevice.SerialNumber()
		if err != nil {
			return fmt.Errorf("failed to get serial number of device at index %d: %v", i, ret)
		}

		pciBus, ret := hlmlDevice.PCIBusID()
		if ret != nil {
			return fmt.Errorf("failed to get PCI bus ID of device at index %d: %v", i, ret)
		}

		pciIdHex, ret := hlmlDevice.PCIID()
		if ret != nil {
			return fmt.Errorf("failed to get PCI ID of device at index %d: %v", i, ret)
		}
		pciId := fmt.Sprintf("%x", pciIdHex)

		klog.V(5).Infof("HLML: found device: serial %v, PCI bus %v, PCI ID %v\n", serial, pciBus, pciId)

		// hlml.Device.PCIID has both vendor and device ID, but device ID has no '0x' prefix.
		uid := device.DeviceUIDFromPCIinfo(pciBus, fmt.Sprintf("0x%v", pciId[4:]))
		gaudi, found := d.state.allocatable[uid]
		if !found {
			return fmt.Errorf("could not find device with UID %v", uid)
		}

		gaudi.Serial = serial
	}

	return nil
}

// monitorHealth spawns a single Go routine to watch for events that
// might signal about device becoming unusable. If such an event
// happens, the ResourceSlice will be updated with kubernetes.io/healthy
// attribute set false.
// See https://github.com/kubernetes/kubernetes/issues/128979
//
// TODO: use KEP-5055: DRA: device taints and tolerations, when it is implemented.
func (d *driver) startHealthMonitor(ctx context.Context) {
	// Watch for device UIDs to mark unhealthy.
	idsChan := make(chan string)
	hlmlContext, stopHLMLMonitor := context.WithCancel(ctx)
	go d.watchCriticalHLMLEvents(hlmlContext, healthCheckIntervalSeconds, idsChan)

	for {
		select {
		// Listen to original ctx, when driver is shutting down, stop HLML watcher.
		case <-ctx.Done():
			stopHLMLMonitor()
			return
		case unhealthyUID := <-idsChan:
			d.updateHealth(hlmlContext, false, unhealthyUID)
		}
	}
}

// updateHealth is called from healthMonitor to change device health flag and
// publish updated resource slice.
func (d *driver) updateHealth(ctx context.Context, healthy bool, uid string) {
	d.state.Lock()
	defer d.state.Unlock()

	d.state.allocatable[uid].Healthy = healthy
	// Health is updated from a go routine, nothing we can do when publishing
	// resource slice fails, so error is ignored.
	if err := d.PublishResourceSlice(ctx); err != nil {
		klog.Errorf("could not publish updated resoruce slice: %v", err)
	}
}

// watchCriticalHLMLEvents watches for critical events from HLML and marks the devices as unhealthy.
func (d *driver) watchCriticalHLMLEvents(ctx context.Context, intervalSeconds int, idsChan chan<- string) {
	eventSet := hlml.NewEventSet()
	defer hlml.DeleteEventSet(eventSet)

	for _, d := range d.state.allocatable {
		err := hlml.RegisterEventForDevice(eventSet, hlml.HlmlCriticalError, d.Serial)
		if err != nil {
			klog.Error("Failed registering critial event for device. Marking it unhealthy", "UID", d.UID, "error", err)
			idsChan <- d.UID
			continue
		}
	}

	healthCheckInterval := time.NewTicker(time.Duration(intervalSeconds) * time.Second)

	for {
		select {
		case <-ctx.Done():
			return
		case <-healthCheckInterval.C:
			e, err := hlml.WaitForEvent(eventSet, 1000)
			if err != nil {
				klog.Errorf("HLML WaitForEvent failed: %v", err)
				time.Sleep(2 * time.Second)
				continue
			}

			klog.V(5).Infof("HLML event received: %+v", e)

			if e.Etype != hlml.HlmlCriticalError {
				klog.V(5).Infof("Ignoring unexpected non-critical HLML error event: %+v", e)
				continue
			}

			dev, err := hlml.DeviceHandleBySerial(e.Serial)
			if err != nil {
				klog.Error("critical: could not get device handle by serial. All devices will go unhealthy", "event", e.Etype)
				// All devices are unhealthy
				for _, d := range d.state.allocatable {
					idsChan <- d.UID
				}
				continue
			}

			serial, err := dev.SerialNumber()
			if err != nil || len(serial) == 0 {
				klog.Error("critical: could not get serial. All devices will go unhealthy", "event", e.Etype)
				// All devices are unhealthy
				for _, d := range d.state.allocatable {
					idsChan <- d.UID
				}
				continue
			}

			for deviceUID, d := range d.state.allocatable {
				if d.Serial == serial {
					klog.Error("critical: the device is unhealthy", "UID", deviceUID, "xid", e.Etype, "serial", d.Serial)
					idsChan <- d.UID
				}
			}
		}
	}
}

func (d *driver) Shutdown(ctx context.Context) error {
	d.plugin.Stop()
	if d.hlmlShutdown != nil {
		d.hlmlShutdown()

		time.Sleep(1 * time.Second)

		ret := hlml.Shutdown()
		if ret != nil {
			klog.Errorf("failed to shutdown HLML: %v", ret)
		}
	}

	return nil
}
