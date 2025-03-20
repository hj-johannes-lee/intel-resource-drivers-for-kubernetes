/* SPDX-License-Identifier: MIT
 *
 * Copyright (c) 2024, Intel Corporation. All Rights Reserved.
 *
 */

#ifndef __FAKE_HLML_H__
#define __FAKE_HLML_H__

#ifdef __cplusplus
extern "C" {
#endif

#include "../../vendor/github.com/HabanaAI/gohlml/hlml.h"

#define DEVICES_MAX              8
#define FAKE_EVENTS_MAX          8
#define NAME_MAX                 64
#define SERIAL_MAX               64
#define FAKE_CALLS_MAX           14

/* Enum for returned values of the different APIs */
typedef enum call_identity {
	FAKE_INIT = 0,
	FAKE_INIT_WITH_FLAGS = 1,
	FAKE_SHUTDOWN = 2,
    FAKE_DEVICE_GET_COUNT = 3,
    FAKE_DEVICE_GET_HANDLE_BY_PCI_BUS_ID = 4,
    FAKE_DEVICE_GET_HANDLE_BY_INDEX = 5,
    FAKE_DEVICE_GET_HANDLE_BY_UUID = 6,
    FAKE_DEVICE_GET_NAME = 7,
    FAKE_DEVICE_GET_PCI_INFO = 8,
    FAKE_DEVICE_GET_SERIAL = 9,
    FAKE_DEVICE_REGISTER_EVENTS = 10,
    FAKE_EVENT_SET_CREATE = 11,
    FAKE_EVENT_SET_FREE = 12,
    FAKE_EVENT_SET_WAIT = 13,
} call_identity_t;

void add_device(const char *pci_addr, const char *pci_device_id, const char *pci_vendor_id, const char *serial, unsigned int index);
void reset(void);

void set_error(call_identity_t call_id, hlml_return_t errCode);
void set_success(call_identity_t call_id);

void add_critical_event(const char *serial);
void reset_events(void);

#ifdef __cplusplus
}   //extern "C"
#endif

#endif /* __FAKE_HLML_H__ */
