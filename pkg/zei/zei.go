package zei

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	zeiAPIBaseURL = "https://api.timeular.com/api/v2"
	TimeFormat    = "2006-01-02T15:04:05.000"
)

func apiURL(path string) string {
	return fmt.Sprintf("%s/%s", zeiAPIBaseURL, path)
}

// Client is a Zei API client.
type Client struct {
	http *http.Client
}

// NewClient returns an initialized client.
func NewClient() *Client {
	return &Client{
		// FIXME: initialize the http client properly
		http: &http.Client{},
	}
}

type developerSignInRequest struct {
	APIKey    string `json:"apiKey"`
	APISecret string `json:"apiSecret"`
}

type developerSignInResponse struct {
	Token string `json:"token"`
}

func (c *Client) authorize(req *http.Request, token string) {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
}

// DeveloperSignIn obtains an access token for an API key and secret
// pair.
func (c *Client) DeveloperSignIn(
	ctx context.Context,
	apiKey, apiSecret string,
) (string, error) {
	reqBody, err := json.Marshal(developerSignInRequest{
		APIKey:    apiKey,
		APISecret: apiSecret,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, apiURL("/developer/sign-in"), bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}

	res, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	var apiResponse developerSignInResponse

	err = json.NewDecoder(res.Body).Decode(&apiResponse)
	if err != nil {
		return "", err
	}

	return apiResponse.Token, nil
}

// Activity is one of the core concepts of ZEI, they are what time is
// tracked on.
type Activity struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Integration string `json:"integration"`
	DeviceSide  int    `json:"deviceSide"`
}

func (c *Client) AssignActivity(
	ctx context.Context,
	token string,
	activityID string,
	deviceSide int,
) (*Activity, error) {
	req, err := http.NewRequest(http.MethodPost, apiURL(fmt.Sprintf("/activities/%s/device-side/%d", activityID, deviceSide)), nil)
	if err != nil {
		return nil, err
	}
	c.authorize(req, token)

	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("ZEI API response status %d", res.StatusCode)
	}

	var activity Activity
	err = json.NewDecoder(res.Body).Decode(&activity)
	if err != nil {
		return nil, err
	}

	return &activity, nil
}

type activitiesResponse struct {
	Activities []Activity `json:"activities"`
}

// Activities returns the list of registered activies
func (c *Client) Activities(
	ctx context.Context,
	accessToken string,
) ([]Activity, error) {
	var (
		activities  []Activity
		apiResponse activitiesResponse
		err         error
	)

	req, err := http.NewRequest(http.MethodGet, apiURL("/activities"), nil)
	if err != nil {
		return activities, err
	}
	c.authorize(req, accessToken)
	req.Header.Set("Content-Type", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return activities, err
	}
	defer res.Body.Close()

	err = json.NewDecoder(res.Body).Decode(&apiResponse)
	if err != nil {
		return activities, err
	}

	return apiResponse.Activities, nil
}

// Tracking is the time tracking information of an activity
type Tracking struct {
	Activity  Activity `json:"activity"`
	StartedAt string   `json:"startedAt"`
}

type currentTrackingResponse struct {
	CurrentTracking Tracking
}

// CurrentTracking returns the current time tracking
func (c *Client) CurrentTracking(
	ctx context.Context,
	accessToken string,
) (*Tracking, error) {
	var (
		apiResponse currentTrackingResponse
		err         error
	)

	req, err := http.NewRequest(
		http.MethodGet,
		apiURL("/tracking"),
		nil,
	)
	if err != nil {
		return nil, err
	}
	c.authorize(req, accessToken)

	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	err = json.NewDecoder(res.Body).Decode(&apiResponse)
	if err != nil {
		return nil, err
	}

	return &apiResponse.CurrentTracking, nil
}

type startTrackingRequest struct {
	StartedAt string `json:"startedAt"`
}

// StartTracking starts time tracking of an activity
func (c *Client) StartTracking(
	ctx context.Context,
	accessToken string,
	activityID string,
	startedAt time.Time,
) error {
	reqBody, err := json.Marshal(&startTrackingRequest{
		StartedAt: startedAt.UTC().Format(TimeFormat),
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(
		http.MethodPost,
		apiURL(fmt.Sprintf("/tracking/%s/start", activityID)),
		bytes.NewReader(reqBody),
	)
	if err != nil {
		return err
	}
	c.authorize(req, accessToken)
	req.Header.Set("Content-Type", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	return nil
}

type stopTrackingRequest struct {
	StoppedAt string `json:"stoppedAt"`
}

// StopTracking stops time tracking of an activity
func (c *Client) StopTracking(
	ctx context.Context,
	accessToken string,
	activityID string,
	stoppedAt time.Time,
) error {
	reqBody, err := json.Marshal(&stopTrackingRequest{
		StoppedAt: stoppedAt.UTC().Format(TimeFormat),
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(
		http.MethodPost,
		apiURL(fmt.Sprintf("/tracking/%s/stop", activityID)),
		bytes.NewReader(reqBody),
	)
	if err != nil {
		return err
	}
	c.authorize(req, accessToken)
	req.Header.Set("Content-Type", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	return nil
}
