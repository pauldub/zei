// +build linux

package main

import (
	"github.com/go-ble/ble"
	"github.com/go-ble/ble/linux"
)

func getDevice() (ble.Device, error) {
	return linux.NewDevice()
}
