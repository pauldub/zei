// +build darwin

package main

import (
	"github.com/currantlabs/ble"
	"github.com/currantlabs/ble/darwin"
)

func getDevice() (ble.Device, error) {
	return darwin.NewDevice()
}
