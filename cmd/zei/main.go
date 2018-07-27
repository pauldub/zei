package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/pauldub/zei/rpc/zeid"
	"github.com/alecthomas/kingpin"
)

var (
	app        = kingpin.New("zei", "A ZEI Timeular command line client.")
	apiAddress = app.Flag("api", "Address to the API server.").Default("http://localhost:8594").String()

	status         = app.Command("status", "Prints Timeular status on stdout.")
	listActivities = app.Command("activities", "List Timeular activities.")

	assignActivity   = app.Command("assign", "Assigns an activity to a device side.")
	assignActivityID = assignActivity.Flag("id", "The ID of the activity to assign.").Required().String()
)

func main() {
	command, err := app.Parse(os.Args[1:])
	if err != nil {
		log.Printf("failed to parse arguments: %+v", err)
		os.Exit(1)
	}

	client := zeid.NewZeiProtobufClient(*apiAddress, &http.Client{})

	switch kingpin.MustParse(command, err) {
	case status.FullCommand():
		ctx := context.Background()

		currentActivity, err := client.CurrentActivity(ctx, &zeid.CurrentActivityReq{})
		logError("failed to request current activity", err)

		startTime, err := time.Parse(time.RFC3339, currentActivity.StartTime)
		logError("failed to parse startTime", err)

		if currentActivity.IsIdle {
			fmt.Println("Not tracking anything!")
			os.Exit(0)
		}

		fmt.Printf("Tracking %s since %s\n", currentActivity.Activity.Name, time.Since(startTime).Truncate(time.Second).String())
		os.Exit(0)
	case listActivities.FullCommand():
		ctx := context.Background()

		res, err := client.ListActivities(ctx, &zeid.ListActivitiesReq{})
		logError("failed to request activities", err)

		fmt.Println("Activities:")

		for _, a := range res.Activities {
			if res.CurrentActivityId == a.Id {
				fmt.Printf("%s - %s (on top)\n", a.Id, a.Name)
			} else {
				fmt.Printf("%s - %s\n", a.Id, a.Name)
			}
		}

		os.Exit(0)
	case assignActivity.FullCommand():
		ctx := context.Background()

		_, err := client.AssignActivity(ctx, &zeid.AssignActivityReq{
			ActivityId: *assignActivityID,
		})
		logError("failed to assign activity", err)

		fmt.Printf("Activity %s was succesfully assigned!\n", *assignActivityID)
		os.Exit(0)
	}
}

func logError(message string, err error) {
	if err != nil {
		log.Printf("%s: %+v", message, err)
		os.Exit(1)
	}
}
