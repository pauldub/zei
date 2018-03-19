// +build darwin

package main

import (
	"github.com/go-ble/ble"
	"github.com/go-ble/ble/darwin"
)

func getDevice() (ble.Device, error) {
	return darwin.NewDevice()
}
