/* Copyright (C) 2024 Intel Corporation
 * SPDX-License-Identifier: Apache-2.0
 */

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/dynamic-resource-allocation/kubeletplugin"
	"k8s.io/klog/v2"
)

func main() {
	var (
		err error
		d   *driver
	)

	klog.Infof("DRA kubelet plugin %s", driverName)

	ctx := context.Background()

	if err = os.MkdirAll(driverPluginPath, 0750); err != nil {
		klog.Errorf("Could not create '%s': %v", driverPluginPath, err)
		return
	}

	if d, err = newDriver(ctx); err != nil {
		klog.Errorf("failed to create kubelet plugin driver: %v", err)
		return
	}

	plugin, err := kubeletplugin.Start(
		ctx,
		d,
		kubeletplugin.KubeClient(d.kubeclient),
		kubeletplugin.NodeName(d.nodename),
		kubeletplugin.DriverName(driverName),
		kubeletplugin.RegistrarSocketPath(pluginRegistrationPath),
		kubeletplugin.PluginSocketPath(driverPluginSocketPath),
		kubeletplugin.KubeletPluginSocketPath(driverPluginSocketPath))
	if err != nil {
		klog.Errorf("failed to start kubelet plugin: %v", err)
		return
	}

	d.plugin = plugin

	d.UpdateDeviceResources(ctx)

	klog.Infof("DRA kubelet plugin %s running...", driverName)

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-sigc

	plugin.Stop()

	klog.Infof("DRA kubelet plugin %s done", driverName)
}
