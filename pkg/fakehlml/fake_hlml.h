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

void add_device(const char *pci_addr, const char *pci_device_id, const char *pci_vendor_id, const char *serial, unsigned int index);

void reset(void);

#ifdef __cplusplus
}   //extern "C"
#endif

#endif /* __FAKE_HLML_H__ */
