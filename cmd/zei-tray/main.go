package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/pauldub/zei/rpc/zeid"
	"github.com/getlantern/systray"
)

var (
	apiAddress = flag.String("api-addr", "http://localhost:8594", "zeid API address (default: 'http://localhost:8594')")
)

func main() {
	flag.Parse()

	client := zeid.NewZeiProtobufClient(*apiAddress, &http.Client{})

	log.Printf("start")
	systray.Run(onReady(client), onExit)
}

func onReady(client zeid.Zei) func() {
	return func() {
		log.Printf("ready")
		systray.SetTitle("zei")
		systray.SetIcon(Icon)
		systray.SetTooltip("hello")

		updateMenu(client)
	}
}

func updateMenu(client zeid.Zei) {
	ctx := context.Background()

	currentActivity, err := client.CurrentActivity(ctx, &zeid.CurrentActivityReq{})
	if err != nil {
		log.Printf("failed to get current activity: %+v", err)
	}

	startTime, err := time.Parse(time.RFC3339, currentActivity.StartTime)
	if err != nil {
		log.Printf("failed to parse startTime: %+v", err)
	}

	systray.AddSeparator()

	currentActivityMenu := systray.AddMenuItem("Not tracking", "")
	if currentActivity.Activity != nil {
		currentActivityMenu.SetTitle(formatCurrentActivity(currentActivity.Activity, startTime, currentActivity.IsIdle))

	}

	systray.AddSeparator()

	quitMenu := systray.AddMenuItem("Quit", "Quit zei-tray")
	go func() {
		<-quitMenu.ClickedCh
		systray.Quit()
	}()

	for _ = range time.Tick(time.Second) {
		currentActivity, err := client.CurrentActivity(ctx, &zeid.CurrentActivityReq{})
		if err != nil {
			log.Printf("failed to get current activity: %+v", err)
		} else {
			startTime, err := time.Parse(time.RFC3339, currentActivity.StartTime)
			if err != nil {
				log.Printf("failed to parse startTime: %+v", err)
			}

			currentActivityMenu.SetTitle(formatCurrentActivity(
				currentActivity.Activity,
				startTime,
				currentActivity.IsIdle,
			))
		}
	}
}

func formatCurrentActivity(a *zeid.Activity, startTime time.Time, idle bool) string {
	if idle {
		return "Not tracking"
	}

	return fmt.Sprintf("%s - %s", a.Name, time.Since(startTime).Truncate(time.Second).String())
}

func onExit() {
}
