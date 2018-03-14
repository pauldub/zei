// +build linux

package main

import (
	"github.com/currantlabs/ble"
	"github.com/currantlabs/ble/linux"
)

func getDevice() (ble.Device, error) {
	return linux.NewDevice()
}
