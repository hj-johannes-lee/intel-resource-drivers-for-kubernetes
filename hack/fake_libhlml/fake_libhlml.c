/* SPDX-License-Identifier: Apache-2.0
 *
 * Copyright (C) 2025, Intel Corporation.
 * All Rights Reserved.
 *
 */

#include "../../vendor/github.com/HabanaAI/gohlml/hlml.h"
#include <stdio.h>
#include <string.h>

/*
type hlmlInterface interface {
    InitWithLogs()
    NewEventSet()
    DeleteEventSet(eventSet)
    RegisterEventForDevice(eventSet, hlml.HlmlCriticalError, d.Serial)
    WaitForEvent(eventSet, 1000)
    Shutdown()
}
*/

#define MAX_DEVICES 8
#define NAME_MAX    64
#define SERIAL_MAX  64

struct device_info_t {
       char pci_addr[PCI_ADDR_LEN];
       unsigned int device_id;
       unsigned int vendor_id;
       char serial[SERIAL_MAX];
       unsigned int index;
};

struct main_struct_t {
    bool initialized;
    int devices_num;
    struct device_info_t devices_info[MAX_DEVICES];
} main_struct;

// custom_init is called from a test function to populate the main_struct
// with fake information that otherwise would have been deduced from the
// sysfs by a real HLML library.
void add_device(const char *pci_addr, const char *pci_device_id, const char *pci_vendor_id,
                const char *serial, unsigned int index) {
    if (pci_addr) {
        snprintf(main_struct.devices_info[main_struct.devices_num].pci_addr, PCI_ADDR_LEN, "%s", pci_addr);
    } else {
        main_struct.devices_info[main_struct.devices_num].pci_addr[0] = '\0';
    }

    if (serial) {
        snprintf(main_struct.devices_info[main_struct.devices_num].serial, SERIAL_MAX, "%s", serial);
    } else {
        main_struct.devices_info[main_struct.devices_num].serial[0] = '\0';
    }

    main_struct.devices_info[main_struct.devices_num].index = index;
    sscanf(pci_device_id, "%x", &main_struct.devices_info[main_struct.devices_num].device_id);
    sscanf(pci_vendor_id, "%x", &main_struct.devices_info[main_struct.devices_num].vendor_id);
    main_struct.devices_num ++;
};

void reset() {
    main_struct.initialized = false;
    main_struct.devices_num = 0;
    return;
};

/* supported APIs */
hlml_return_t hlml_init(void) {
    printf("hlml_init called\n");
    return hlml_init_with_flags(0);
};

hlml_return_t hlml_init_with_flags(unsigned int flags) {
    printf("hlml_init_with_flags called\n");
    main_struct.initialized = true;

    return HLML_SUCCESS;
};

hlml_return_t hlml_shutdown(void) {
    printf("hlml_shutdown called\n");
    main_struct.initialized = false;

    return HLML_SUCCESS;
};

hlml_return_t hlml_device_get_count(unsigned int *device_count) {
    printf("hlml_device_get_count called\n");
    if (!device_count)
        return HLML_ERROR_INVALID_ARGUMENT;
    *device_count = main_struct.devices_num;
    return HLML_SUCCESS;
};

hlml_return_t hlml_device_get_handle_by_pci_bus_id(const char *pci_addr, hlml_device_t *device) {
    printf("hlml_device_get_handle_by_pci_bus_id called\n");

    struct device_info_t *device_info;

    if (!main_struct.initialized) {
        return HLML_ERROR_UNINITIALIZED;
    }

    if (!device || !pci_addr)
        return HLML_ERROR_INVALID_ARGUMENT;

    for (int n = 0; n < main_struct.devices_num; n++) {
        device_info = &main_struct.devices_info[n];
        if (strncmp(pci_addr, device_info->pci_addr, PCI_ADDR_LEN)) {
            *device = device_info;

            return HLML_SUCCESS;
        }
    }
    return HLML_ERROR_NOT_FOUND;
};

hlml_return_t hlml_device_get_handle_by_index(unsigned int index, hlml_device_t *device) {
    printf("hlml_device_get_handle_by_index called\n");

    struct device_info_t *device_info;

    if (!main_struct.initialized) {
        return HLML_ERROR_UNINITIALIZED;
    }

    if (!device || ((int)index >= main_struct.devices_num))
        return HLML_ERROR_INVALID_ARGUMENT;

    for (int n = 0; n < main_struct.devices_num; n++) {
        device_info = &main_struct.devices_info[n];
        if (index == device_info->index) {
            *device = device_info;
            return HLML_SUCCESS;
        }
    }
    return HLML_ERROR_NOT_FOUND;
};

hlml_return_t hlml_device_get_handle_by_UUID (const char* uuid, hlml_device_t *device) {
    printf("hlml_device_get_handle_by_UUID called\n");
    return HLML_SUCCESS;
};

hlml_return_t hlml_device_get_name(hlml_device_t device, char *name,
                   unsigned int  length) {
    printf("hlml_device_get_name called\n");
    return HLML_SUCCESS;
};

hlml_return_t hlml_device_get_pci_info(hlml_device_t device, hlml_pci_info_t *pci) {
    printf("hlml_device_get_pci_info called\n");

    if (!main_struct.initialized) {
        return HLML_ERROR_UNINITIALIZED;
    }

	if (!device || !pci)
		return HLML_ERROR_INVALID_ARGUMENT;

    struct device_info_t *device_info = (struct device_info_t *)device;

    strncpy(pci->bus_id, device_info->pci_addr, PCI_ADDR_LEN);
	pci->bus_id[PCI_ADDR_LEN - 1] = '\0';

	pci->pci_device_id = device_info->device_id | (device_info->vendor_id << 16);

    return HLML_SUCCESS;
};

hlml_return_t hlml_device_get_clock_info(hlml_device_t device,
                     hlml_clock_type_t type,
                     unsigned int *clock) {
    printf("hlml_device_get_clock_info called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_max_clock_info(hlml_device_t device,
                         hlml_clock_type_t type,
                         unsigned int *clock) {
    printf("hlml_device_get_max_clock_info called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_clock_limit_info(hlml_device_t device,
                                            hlml_clock_type_t type,
                                            unsigned int *clock) {
    printf("hlml_device_get_clock_limit_info called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_utilization_rates(hlml_device_t device,
                    hlml_utilization_t *utilization) {
    printf("hlml_device_get_utilization_rates called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_memory_info(hlml_device_t device, hlml_memory_t *memory) {
    printf("hlml_device_get_memory_info called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_temperature(hlml_device_t device,
                      hlml_temperature_sensors_t sensor_type,
                      unsigned int *temp) {
    printf("hlml_device_get_temperature called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_temperature_threshold(hlml_device_t device,
                hlml_temperature_thresholds_t threshold_type,
                unsigned int *temp) {
    printf("hlml_device_get_temperature_threshold called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_persistence_mode(hlml_device_t device,
                        hlml_enable_state_t *mode) {
    printf("hlml_device_get_persistence_mode called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_performance_state(hlml_device_t device,
                        hlml_p_states_t *p_state) {
    printf("hlml_device_get_performance_state called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_supported_performance_states(hlml_device_t device,
                                                       hlml_p_states_t *pstates,
                                                       unsigned int size) {
    printf("hlml_device_get_supported_performance_states called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_power_usage(hlml_device_t device,
                      unsigned int *power) {
    printf("hlml_device_get_power_usage called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_power_management_mode(hlml_device_t device,
                                                   hlml_enable_state_t *state) {
    printf("hlml_device_get_power_management_mode called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_power_management_limit(hlml_device_t device,
                                                    unsigned int *limit) {
    printf("hlml_device_get_power_management_limit called\n");
    return HLML_ERROR_NOT_SUPPORTED;
}

hlml_return_t hlml_device_set_power_management_limit(hlml_device_t device, unsigned int limit) {
    printf("hlml_device_set_power_management_limit called\n");
    return HLML_ERROR_NOT_SUPPORTED;
}

hlml_return_t hlml_device_get_power_management_default_limit(hlml_device_t device,
                        unsigned int *default_limit) {
    printf("hlml_device_get_power_management_default_limit called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_ecc_mode(hlml_device_t device,
                       hlml_enable_state_t *current,
                       hlml_enable_state_t *pending) {
    printf("hlml_device_get_ecc_mode called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_total_ecc_errors(hlml_device_t device,
                    hlml_memory_error_type_t error_type,
                    hlml_ecc_counter_type_t counter_type,
                    unsigned long long *ecc_counts) {
    printf("hlml_device_get_total_ecc_errors called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_memory_error_counter(hlml_device_t device,
                    hlml_memory_error_type_t error_type,
                    hlml_ecc_counter_type_t counter_type,
                    hlml_memory_location_type_t location,
                    unsigned long long *ecc_counts) {
    printf("hlml_device_get_memory_error_counter called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_uuid(hlml_device_t device,
                   char *uuid,
                   unsigned int length) {
    printf("hlml_device_get_uuid called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_minor_number(hlml_device_t device,
                       unsigned int *minor_number) {
    printf("hlml_device_get_minor_number called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

// TODO: implement registration and events sending
hlml_return_t hlml_device_register_events(hlml_device_t device,
                      unsigned long long event_types,
                      hlml_event_set_t set) {
    printf("hlml_device_register_events called\n");
    return HLML_ERROR_INVALID_ARGUMENT;
};

// TODO: implement registration and events sending
hlml_return_t hlml_event_set_create(hlml_event_set_t *set) {
    printf("hlml_event_set_create called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

// TODO: implement registration and events sending
hlml_return_t hlml_event_set_free(hlml_event_set_t set) {
    printf("hlml_event_set_free called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_event_set_wait(hlml_event_set_t set,
                  hlml_event_data_t *data,
                  unsigned int timeoutms) {
    printf("hlml_event_set_wait called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_mac_info(hlml_device_t device,
                       hlml_mac_info_t *mac_info,
                       unsigned int mac_info_size,
                       unsigned int start_mac_id,
                       unsigned int *actual_mac_count) {
    printf("hlml_device_get_mac_info called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_hl_revision(hlml_device_t device, int *hl_revision) {
    printf("hlml_device_get_hl_revision called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_pcb_info(hlml_device_t device, hlml_pcb_info_t *pcb) {
    printf("hlml_device_get_pcb_info called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_serial(hlml_device_t device, char *serial, unsigned int length) {
    printf("hlml_device_get_serial called\n");

    if (SERIAL_MAX > (int)length) {
		return HLML_ERROR_INSUFFICIENT_SIZE;
	}

    struct device_info_t *device_info = (struct device_info_t *)device;
	strncpy(serial, device_info->serial, length);

    return HLML_SUCCESS;
};

hlml_return_t hlml_device_get_module_id(hlml_device_t device, unsigned int *module_id) {
    printf("hlml_device_get_module_id called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_board_id(hlml_device_t device, unsigned int* board_id) {
    printf("hlml_device_get_board_id called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_pcie_throughput(hlml_device_t device,
                          hlml_pcie_util_counter_t counter,
                          unsigned int *value) {
    printf("hlml_device_get_pcie_throughput called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_pcie_replay_counter(hlml_device_t device, unsigned int *value) {
    printf("hlml_device_get_pcie_replay_counter called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_curr_pcie_link_generation(hlml_device_t device,
                            unsigned int *curr_link_gen) {
    printf("hlml_device_get_curr_pcie_link_generation called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_curr_pcie_link_width(hlml_device_t device,
                           unsigned int *curr_link_width) {
    printf("hlml_device_get_curr_pcie_link_width called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_current_clocks_throttle_reasons(hlml_device_t device,
        unsigned long long *clocks_throttle_reasons) {
    printf("hlml_device_get_current_clocks_throttle_reasons called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_total_energy_consumption(hlml_device_t device,
        unsigned long long *energy) {
    printf("hlml_device_get_total_energy_consumption called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_get_mac_addr_info(hlml_device_t device, uint64_t *mask, uint64_t *ext_mask) {
    printf("hlml_get_mac_addr_info called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_nic_get_link(hlml_device_t device, uint32_t port, bool *up) {
    printf("hlml_nic_get_link called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_nic_get_statistics(hlml_device_t device, hlml_nic_stats_info_t *stats_info) {
    printf("hlml_nic_get_statistics called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_clear_cpu_affinity(hlml_device_t device) {
    printf("hlml_device_clear_cpu_affinity called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_cpu_affinity(hlml_device_t device,
                       unsigned int cpu_set_size,
                       unsigned long *cpu_set) {
    printf("hlml_device_get_cpu_affinity called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_cpu_affinity_within_scope(hlml_device_t device,
                            unsigned int cpu_set_size,
                            unsigned long *cpu_set,
                            hlml_affinity_scope_t scope) {
    printf("hlml_device_get_cpu_affinity_within_scope called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_memory_affinity(hlml_device_t device,
                          unsigned int node_set_size,
                          unsigned long *node_set,
                          hlml_affinity_scope_t scope) {
    printf("hlml_device_get_memory_affinity called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_set_cpu_affinity(hlml_device_t device) {
    printf("hlml_device_set_cpu_affinity called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_violation_status(hlml_device_t device,
                           hlml_perf_policy_type_t perf_policy_type,
                           hlml_violation_time_t *viol_time) {
    printf("hlml_device_get_violation_status called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_replaced_rows(hlml_device_t device,
                        hlml_row_replacement_cause_t cause,
                        unsigned int *row_count,
                        hlml_row_address_t *addresses) {
    printf("hlml_device_get_replaced_rows called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_replaced_rows_pending_status(hlml_device_t device,
                               hlml_enable_state_t *is_pending) {
    printf("hlml_device_get_replaced_rows_pending_status called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_get_hlml_version(char *version, unsigned int length) {
    printf("hlml_get_hlml_version called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_get_driver_version(char *driver_version, unsigned int length) {
    printf("hlml_get_driver_version called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_get_nic_driver_version(char *driver_version, unsigned int length) {
    printf("hlml_get_nic_driver_version called\n");
    return HLML_ERROR_NOT_SUPPORTED;
}

hlml_return_t hlml_get_model_number(hlml_device_t device, char *model_number,
                    unsigned int length) {
    printf("hlml_get_model_number called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_get_firmware_fit_version(hlml_device_t device, char *firmware_fit,
                        unsigned int length) {
    printf("hlml_get_firmware_fit_version called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_get_firmware_spi_version(hlml_device_t device, char *firmware_spi,
                        unsigned int length) {
    printf("hlml_get_firmware_spi_version called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_get_fw_boot_version(hlml_device_t device, char *fw_boot_version,
                       unsigned int length) {
    printf("hlml_get_fw_boot_version called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_get_fw_os_version(hlml_device_t device, char *fw_os_version,
                     unsigned int length) {
    printf("hlml_get_fw_os_version called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_get_cpld_version(hlml_device_t device, char *cpld_version,
                    unsigned int length) {
    printf("hlml_get_cpld_version called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};

hlml_return_t hlml_device_get_oper_status(hlml_device_t device, char *status,
                                         unsigned int length) {
    printf("hlml_device_get_oper_status called\n");
    return HLML_ERROR_NOT_SUPPORTED;
};
