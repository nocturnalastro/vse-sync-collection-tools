// Copyright 2023 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package devices

import (
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/redhat-partner-solutions/vse-sync-testsuite/pkg/clients"
)

type PTPDeviceInfo struct {
	VendorID string
	DeviceID string
	TtyGNSS  string
}

type DevDPLLInfo struct {
	State  string
	Offset string
}
type GNSSTTYLines struct {
	TTY   string
	Lines string
}

func GetPTPDeviceInfo(interfaceName string, ctx clients.ContainerContext) (devInfo PTPDeviceInfo) {
	// Find the tty for the GNSS for this interface
	GNSStty := commandWithPostprocessFunc(ctx, strings.TrimSpace, []string{
		"ls", "/sys/class/net/" + interfaceName + "/device/gnss/",
	})

	devInfo.TtyGNSS = "/dev/" + GNSStty
	// "/dev/" + busToGNSS(busID)
	// log.Debugf("got busID for %s:  %s", interfaceName, busID)
	// log.Debugf("got tty for %s:  %s", interfaceName, devInfo.TtyGNSS)

	// expecting a string like 0x1593
	devInfo.DeviceID = commandWithPostprocessFunc(ctx, strings.TrimSpace, []string{
		"cat", "/sys/class/net/" + interfaceName + "/device/device",
	})
	log.Debugf("got deviceID for %s:  %s", interfaceName, devInfo.DeviceID)

	// expecting a string like 0x8086
	devInfo.VendorID = commandWithPostprocessFunc(ctx, strings.TrimSpace, []string{
		"cat", "/sys/class/net/" + interfaceName + "/device/vendor",
	})
	log.Debugf("got vendorID for %s:  %s", interfaceName, devInfo.VendorID)
	return
}

// transform a bus ID to an expected GNSS TTY name.
// e.g. "0000:86:00.0" -> "ttyGNSS_8600", "0000:51:02.1" -> "ttyGNSS_5102"
// func busToGNSS(busID string) string {
// 	log.Debugf("convert %s to GNSS tty", busID)
// 	parts := strings.Split(busID, ":")
// 	ttyGNSS := parts[1] + strings.Split(parts[2], ".")[0]
// 	return "ttyGNSS_" + ttyGNSS
// }

func commandWithPostprocessFunc(ctx clients.ContainerContext, cleanupFunc func(string) string, command []string) (result string) { //nolint:lll // allow slightly long function definition
	clientset := clients.GetClientset()
	stdout, _, err := clientset.ExecCommandContainer(ctx, command)
	if err != nil {
		log.Errorf("command in container failed unexpectedly. context: %v", ctx)
		log.Errorf("command in container failed unexpectedly. command: %v", command)
		log.Errorf("command in container failed unexpectedly. error: %v", err)
		return ""
	}
	return cleanupFunc(stdout)
}

// Read lines from the ttyGNSS of the passed devInfo.
func ReadTtyGNSS(ctx clients.ContainerContext, devInfo PTPDeviceInfo, lines, timeoutSeconds int) GNSSTTYLines {
	content := commandWithPostprocessFunc(ctx, strings.TrimSpace, []string{
		"timeout", strconv.Itoa(timeoutSeconds), "head", "-n", strconv.Itoa(lines), devInfo.TtyGNSS,
	})
	return GNSSTTYLines{
		TTY:   devInfo.TtyGNSS,
		Lines: content,
	}
}

// GetDevDPLLInfo returns the device DPLL info for an interface.
func GetDevDPLLInfo(ctx clients.ContainerContext, interfaceName string) (dpllInfo DevDPLLInfo) {
	dpllInfo.State = commandWithPostprocessFunc(ctx, strings.TrimSpace, []string{
		"cat", "/sys/class/net/" + interfaceName + "/device/dpll_1_state",
	})
	dpllInfo.Offset = commandWithPostprocessFunc(ctx, strings.TrimSpace, []string{
		"cat", "/sys/class/net/" + interfaceName + "/device/dpll_1_offset",
	})
	return
}
