package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/pauldub/zei/pkg/zei"
	"github.com/pauldub/zei/pkg/zeidsvc"
	"github.com/pauldub/zei/rpc/zeid"
	"github.com/0xAX/notificator"
	"github.com/go-ble/ble"
	"github.com/mitchellh/cli"
)

var (
	orientationService        = "c7e70010c84711e681758c89a55d403c"
	orientationCharacteristic = ble.MustParse("c7e70012c84711e681758c89a55d403c")

	zeiSerialNumber = flag.String("serial-number", "", "ZEI device serial number (optional)")
	zeiAPIKey       = flag.String("api-key", "", "ZEI api key")
	zeiAPISecret    = flag.String("api-secret", "", "ZEI api secret")
	showSide        = flag.Bool("show-side", false, "Show activity side in notifications (default: false)")
	apiAddress      = flag.String("api-addr", ":8594", "Address for API to listen on (default: ':8594')")
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

		notify = notificator.New(notificator.Options{
			AppName: "ZEI",
		})
	)

	flag.Parse()

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
		log.Fatalf("failed to initialize get orientation characteristic: %+v", err)
	}

	svc, err := zeidsvc.NewService(
		ctx, *zeiAPIKey, *zeiAPISecret, conn, profile,
	)
	if err != nil {
		log.Fatalf("failed to initialize zeid serrvice: %+v", err)
	}

	err = conn.Subscribe(orientation, true, func(val []byte) {
		side := int(val[0])
		if side < 1 || side > 8 {
			side = 0
		}

		log.Printf("changed side: %+v", side)

		newActivity, ok := svc.GetActivity(int(val[0]))
		if !ok {
			return
		}

		if newActivity.ID == svc.Current().ID {
			return
		}

		notificationText := newActivity.Name
		if *showSide {
			notificationText = fmt.Sprintf("%s (%d)", notificationText, newActivity.DeviceSide)
		}

		if newActivity.Name == "Idle" {
			err = svc.Stop(ctx)
			if err != nil {
				log.Printf("failed to stop tracking of current activity: %+v", err)
			}

			err = notifyStop(notify, svc.Current())
			if err != nil {
				log.Printf("failed to send notification: %+v", err)
			}
		} else {
			if !svc.IsIdle() {
				err = svc.Stop(ctx)
				if err != nil {
					log.Printf("failed to stop tracking of current activity: %+v", err)
				}
			}

			err = svc.Start(ctx, newActivity)
			if err != nil {
				log.Printf("failed to start tracking of new activity: %+v", err)
			}

			err = notifyStart(notify, newActivity)
			if err != nil {
				log.Printf("failed to send notification: %+v", err)
			}
		}

		svc.SetActivity(newActivity)
	})
	if err != nil {
		log.Fatal(err)
	}

	handler := zeid.NewZeiServer(svc, nil)

	mux := http.NewServeMux()
	mux.Handle(zeid.ZeiPathPrefix, handler)

	go func() {
		log.Fatal(http.ListenAndServe(*apiAddress, mux))
	}()

	<-conn.Disconnected()
}
func notifyStart(notify *notificator.Notificator, a zei.Activity) error {
	title := a.Name
	if *showSide {
		title = fmt.Sprintf("%s (%d)", title, a.DeviceSide)
	}

	return notify.Push("Starting activity", title, "", notificator.UR_NORMAL)
}

func notifyStop(notify *notificator.Notificator, a zei.Activity) error {
	return notify.Push("Stopping activity", a.Name, "", notificator.UR_NORMAL)
}
