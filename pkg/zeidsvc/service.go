package zeidsvc

import (
	"context"
	"time"

	"github.com/pauldub/zei/pkg/zei"
	"github.com/pauldub/zei/rpc/zeid"
	"github.com/go-ble/ble"
	"github.com/pkg/errors"
)

var (
	orientationCharacteristic = ble.MustParse("c7e70012c84711e681758c89a55d403c")
	idleActivity              = zei.Activity{
		Name:       "Idle",
		DeviceSide: 0,
	}
)

const (
	// minimum and maximum number of sides supported.
	minSide = 1
	maxSide = 8
)

type ZeiSvc interface {
	zeid.Zei

	Current() zei.Activity
	StartTime() time.Time

	GetCurrentSide() (int, error)

	GetActivity(side int) (zei.Activity, bool)
	SetActivity(zei.Activity)

	Start(ctx context.Context, new zei.Activity) error
	Stop(ctx context.Context) error
	IsIdle() bool
}

type zeisvc struct {
	api           *zei.Client
	token         string
	current       zei.Activity
	startTime     time.Time
	activitiesMap map[int]zei.Activity

	bleConn     ble.Client
	orientation *ble.Characteristic
}

func NewService(
	ctx context.Context,
	apiKey, apiSecret string,
	conn ble.Client,
	profile *ble.Profile,
) (ZeiSvc, error) {
	apiClient := zei.NewClient()

	accessToken, err := apiClient.DeveloperSignIn(ctx, apiKey, apiSecret)
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign-in to ZEI API")
	}

	activities, err := apiClient.Activities(ctx, accessToken)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query ZEI activities")
	}

	activitiesMap := map[int]zei.Activity{0: idleActivity}

	for _, a := range activities {
		activitiesMap[a.DeviceSide] = a
	}
	activitiesMap[0] = idleActivity

	orientation, ok := profile.Find(ble.NewCharacteristic(orientationCharacteristic)).(*ble.Characteristic)
	if !ok {
		return nil, errors.New("could not fiend orientation characteristic")
	}

	// FIXME: avoid duplication with `GetCurrentSide`.
	currentOrientation, err := conn.ReadCharacteristic(orientation)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read device orientation")
	}

	side := int(currentOrientation[0])
	if side < 1 || side > 8 {
		side = 0
	}

	currentActivity, ok := activitiesMap[side]
	if !ok {
		currentActivity = activitiesMap[0]
	}

	currentTracking, err := apiClient.CurrentTracking(ctx, accessToken)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get current tracking")
	}

	if currentTracking.Activity.ID != currentActivity.ID {
		err = apiClient.StopTracking(ctx, accessToken, currentTracking.Activity.ID, time.Now())
		if err != nil {
			return nil, errors.Wrap(err, "failed to stop current tracking activity")
		}

		if currentActivity.Name != "Idle" {
			err = apiClient.StartTracking(ctx, accessToken, currentTracking.Activity.ID, time.Now())
			if err != nil {
				return nil, errors.Wrap(err, "failed to start current activity")
			}
		}
	}

	var startTime time.Time

	for _, a := range activities {
		if a.ID == currentTracking.Activity.ID {
			currentActivity = a
			startTime, err = time.Parse(zei.TimeFormat, currentTracking.StartedAt)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse tracking start time")
			}
			break
		}
	}

	return &zeisvc{
		api:           apiClient,
		token:         accessToken,
		activitiesMap: activitiesMap,
		current:       currentActivity,
		startTime:     startTime,
		bleConn:       conn,
		orientation:   orientation,
	}, nil
}

func (z *zeisvc) CurrentActivity(ctx context.Context, req *zeid.CurrentActivityReq) (*zeid.CurrentActivityResp, error) {
	return &zeid.CurrentActivityResp{
		Activity: &zeid.Activity{
			Id:          z.current.ID,
			Name:        z.current.Name,
			Color:       z.current.Color,
			Integration: z.current.Integration,
			DeviceSide:  int64(z.current.DeviceSide),
		},
		StartTime: z.startTime.Format(time.RFC3339),
		IsIdle:    z.IsIdle(),
	}, nil
}

func (z *zeisvc) ListActivities(ctx context.Context, req *zeid.ListActivitiesReq) (*zeid.ListActivitiesResp, error) {
	activities, err := z.api.Activities(ctx, z.token)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query ZEI activities")
	}

	var res = &zeid.ListActivitiesResp{
		Activities:        make([]*zeid.Activity, 0, len(activities)),
		CurrentActivityId: z.current.ID,
	}

	for _, a := range activities {
		res.Activities = append(res.Activities, &zeid.Activity{
			Id:          a.ID,
			Name:        a.Name,
			Color:       a.Color,
			Integration: a.Integration,
			DeviceSide:  int64(a.DeviceSide),
		})
	}

	return res, nil
}

func (z *zeisvc) AssignActivity(ctx context.Context, req *zeid.AssignActivityReq) (*zeid.AssignActivityResp, error) {
	currentSide, err := z.GetCurrentSide()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read current Timeular side")
	}

	activity, err := z.api.AssignActivity(ctx, z.token, req.ActivityId, currentSide)
	if err != nil {
		return nil, errors.Wrap(err, "failed to assign activity")
	}

	// maintain activitiesMap state consistent
	for key, a := range z.activitiesMap {
		if a.ID == activity.ID {
			delete(z.activitiesMap, key)
		}
	}

	z.activitiesMap[currentSide] = *activity

	return &zeid.AssignActivityResp{}, nil
}

func (z *zeisvc) Start(ctx context.Context, new zei.Activity) error {
	z.startTime = time.Now()
	z.current = new
	return z.api.StartTracking(ctx, z.token, new.ID, z.startTime)
}

func (z *zeisvc) Stop(ctx context.Context) error {
	return z.api.StopTracking(ctx, z.token, z.current.ID, time.Now())
}

func (z *zeisvc) IsIdle() bool {
	return z.current.Name == "Idle"
}

func (z *zeisvc) Current() zei.Activity {
	return z.current
}

func (z *zeisvc) StartTime() time.Time {
	return z.startTime
}

func (z *zeisvc) GetActivity(side int) (zei.Activity, bool) {
	if side == 0 {
		return idleActivity, true
	}

	a, ok := z.activitiesMap[side]
	return a, ok
}

func (z *zeisvc) SetActivity(a zei.Activity) {
	z.current = a
}

func (z *zeisvc) GetCurrentSide() (int, error) {
	currentSide, err := z.bleConn.ReadCharacteristic(z.orientation)
	if err != nil {
		return 0, errors.Wrap(err, "failed to read device orientation")
	}

	side := int(currentSide[0])
	if side < minSide || side > maxSide {
		return 0, nil
	}

	return side, nil
}
