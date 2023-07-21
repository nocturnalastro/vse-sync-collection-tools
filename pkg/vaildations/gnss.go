// SPDX-License-Identifier: GPL-2.0-or-later

package vaildations

import (
	"strings"

	"github.com/redhat-partner-solutions/vse-sync-collection-tools/pkg/collectors/devices"
)

const gnssID = "GNSS Version is valid"

var (
	MinGNSSVersion = "2.20"
)

func NewGNSS(gnss *devices.GPSDetails) *VersionCheck {
	parts := strings.Split(gnss.FirmwareVersion, " ")
	return &VersionCheck{
		id:           gnssID,
		Version:      gnss.FirmwareVersion,
		checkVersion: parts[1],
		minVersion:   MinGNSSVersion,
	}
}