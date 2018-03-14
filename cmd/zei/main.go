package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"git.tymate.com/paul/zei/pkg/zei"
	"github.com/0xAX/notificator"
	"github.com/currantlabs/ble"
	"github.com/mitchellh/cli"
)

var (
	orientationService        = "c7e70010c84711e681758c89a55d403c"
	orientationCharacteristic = ble.MustParse("c7e70012c84711e681758c89a55d403c")

	zeiSerialNumber = flag.String("serial-number", "", "ZEI device serial number")
	zeiAPIKey       = flag.String("api-key", "", "ZEI api key")
	zeiAPISecret    = flag.String("api-secret", "", "ZEI api secret")
	showSide        = flag.Bool("show-side", false, "Show activity side in notifications")
)

func main() {
	var (
		ctx = context.Background()
		ui  = cli.BasicUi{
			Reader:      os.Stdin,
			Writer:      os.Stdout,
			ErrorWriter: os.Stderr,
		}

		connMutex   sync.Mutex
		isConnected bool

		apiClient = zei.NewClient()
		notify    = notificator.New(notificator.Options{
			AppName: "ZEI",
		})
	)

	flag.Parse()

	accessToken, err := apiClient.DeveloperSignIn(ctx, *zeiAPIKey, *zeiAPISecret)
	if err != nil {
		log.Fatalf("failed to sign-in to ZEI API: %+v", err)
	}

	activities, err := apiClient.Activities(ctx, accessToken)
	if err != nil {
		log.Fatalf("failed to query ZEI activities: %+v", err)
	}

	sideActivities := map[int]zei.Activity{
		0: {
			Name: "Idle",
		},
	}

	for _, a := range activities {
		sideActivities[a.DeviceSide] = a
	}

	dev, err := getDevice()
	if err != nil {
		log.Fatal(err)
	}

	ble.SetDefaultDevice(dev)

	conn, err := ble.Connect(ctx, func(a ble.Advertisement) bool {
		connMutex.Lock()
		defer connMutex.Unlock()

		isZei := strings.ToUpper(a.LocalName()) == strings.ToUpper("Timeular ZEI")
		if !isZei || isConnected {
			return false
		}

		serialNumber := string(a.ManufacturerData())
		if *zeiSerialNumber != "" {
			return serialNumber == *zeiSerialNumber
		}

		answer, err := ui.Ask(fmt.Sprintf("Connect to ZEI device %q? (y/n)", serialNumber))
		if err != nil {
			ui.Error(err.Error())
			os.Exit(1)
			return false
		}

		if strings.HasPrefix(answer, "y") {
			isConnected = true
			return true
		}

		return false
	})
	if err != nil {
		log.Fatal(err)
	}
	defer conn.CancelConnection()

	ui.Info("connection to device successful")

	profile, err := conn.DiscoverProfile(true)
	if err != nil {
		log.Fatal(err)
	}

	orientation, ok := profile.Find(ble.NewCharacteristic(orientationCharacteristic)).(*ble.Characteristic)
	if !ok {
		log.Println("could not fiend orientation characteristic")
		return
	}

	currentOrientation, err := conn.ReadCharacteristic(orientation)
	if err != nil {
		log.Fatal(err)
	}
	currentActivity := sideActivities[int(currentOrientation[0])]

	err = conn.Subscribe(orientation, true, func(val []byte) {
		newActivity, ok := sideActivities[int(val[0])]
		if !ok {
			return
		}

		if newActivity.ID == currentActivity.ID {
			return
		}

		notificationText := newActivity.Name
		if *showSide {
			notificationText = fmt.Sprintf("%s (%d)", notificationText, newActivity.DeviceSide)
		}

		notificationTitle := "Starting activity"

		if newActivity.Name == "Idle" {
			notificationText = currentActivity.Name
			notificationTitle = "Stopping activity"
			err = apiClient.StopTracking(ctx, accessToken, currentActivity.ID)
			if err != nil {
				log.Printf("failed to stop tracking of current activity: %+v", err)
			}
		} else {
			err = apiClient.StartTracking(ctx, accessToken, newActivity.ID)
			if err != nil {
				log.Printf("failed to start tracking of new activity: %+v", err)
			}
		}

		err := notify.Push(notificationTitle, notificationText, "", notificator.UR_NORMAL)
		if err != nil {
			log.Printf("failed to send notification: %+v", err)
		}

		currentActivity = newActivity
	})
	if err != nil {
		log.Fatal(err)
	}

	<-conn.Disconnected()
}
