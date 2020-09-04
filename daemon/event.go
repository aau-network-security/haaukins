package daemon

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	pb "github.com/aau-network-security/haaukins/daemon/proto"
	"github.com/aau-network-security/haaukins/store"
	pbc "github.com/aau-network-security/haaukins/store/proto"
	"github.com/aau-network-security/haaukins/svcs/guacamole"
	"github.com/rs/zerolog/log"
)

var (
	NPUserMaxLabs   = 40
	NotAvailableTag = "not available tag, there is already an event which is either running, booked or suspended"
)

// INITIAL POINT OF CREATE EVENT FUNCTION, IT INITIALIZE EVENT AND ADDS EVENTPOOL
func (d *daemon) startEvent(ev guacamole.Event) {
	conf := ev.GetConfig()

	var frontendNames []string
	for _, f := range conf.Lab.Frontends {
		frontendNames = append(frontendNames, f.Image)
	}
	log.Info().
		Str("Name", conf.Name).
		Str("Tag", string(conf.Tag)).
		Int("Available", conf.Available).
		Int("Capacity", conf.Capacity).
		Strs("Frontends", frontendNames).
		Msg("Creating event")

	ev.Start(context.TODO())

	d.eventPool.AddEvent(ev)
}

func (d *daemon) CreateEvent(req *pb.CreateEventRequest, resp pb.Daemon_CreateEventServer) error {
	loggerInstance := &GrpcLogger{resp: resp}
	ctx := context.WithValue(resp.Context(), "grpc_logger", loggerInstance)
	log.Ctx(ctx).
		Info().
		Str("tag", req.Tag).
		Str("name", req.Name).
		Int32("available", req.Available).
		Int32("capacity", req.Capacity).
		Strs("frontends", req.Frontends).
		Strs("exercises", req.Exercises).
		Str("startTime", req.StartTime).
		Str("finishTime", req.FinishTime).
		Msg("create event")

	user, err := getUserFromIncomingContext(ctx)
	if err != nil {
		log.Warn().Msgf("User credentials not found ! %v  ", err)
		return fmt.Errorf("user credentials could not found on context %v", err)
	}

	if user.NPUser && req.Capacity > int32(NPUserMaxLabs) {
		return fmt.Errorf("out of quota for members, you have limited access")
	}

	isEligible, err := d.checkUserQuota(ctx, user.Username)
	if err != nil {
		return err
	}

	if (user.NPUser && isEligible) || !user.NPUser {

		now := time.Now()
		if ReservedSubDomains[strings.ToLower(req.Tag)] {
			return ReservedDomainErr
		}

		uniqueExercisesList := removeDuplicates(req.Exercises)

		tags := make([]store.Tag, len(uniqueExercisesList))
		for i, s := range uniqueExercisesList {
			t, err := store.NewTag(s)
			if err != nil {
				return err
			}
			// check exercise before creating event file
			_, tagErr := d.exercises.GetExercisesByTags(t)
			if tagErr != nil {
				return tagErr
			}
			tags[i] = t
		}
		evtag, _ := store.NewTag(req.Tag)
		finishTime, _ := time.Parse(dbTimeFormat, req.FinishTime)
		startTime, _ := time.Parse(dbTimeFormat, req.StartTime)

		if isInvalidDate(startTime) {
			return fmt.Errorf("invalid startTime format %v", startTime)
		}

		// checking through eventPool is not good enough since booked events are not added to eventpool
		isEventExist, err := d.dbClient.IsEventExists(ctx, &pbc.GetEventByTagReq{
			EventTag: req.Tag,
			// it will take INVERT condition which means that query from
			//Running, Suspended and Booked events
			Status: Closed,
		})
		if err != nil {
			return fmt.Errorf("event does not exist or something is wrong: %v", err)
		}
		if isEventExist.IsExist {
			return fmt.Errorf(NotAvailableTag)
		}

		// difference  in days
		// if there is no difference in days it means event  will be
		// started immediately
		difference := math.Round(startTime.Sub(now).Hours() / 24)
		if difference >= 1 {
			if err := d.bookEvent(ctx, req); err != nil {
				return err
			}
			return nil
		}

		conf := store.EventConfig{
			Name:      req.Name,
			Tag:       evtag,
			Available: int(req.Available),
			Capacity:  int(req.Capacity),

			StartedAt:      &startTime,
			FinishExpected: &finishTime,
			Lab: store.Lab{
				Frontends: d.frontends.GetFrontends(req.Frontends...),
				Exercises: tags,
			},
			Status:    Running,
			CreatedBy: user.Username,
		}

		if err := conf.Validate(); err != nil {
			return err
		}

		_, err = d.eventPool.GetEvent(evtag)

		if err == nil {
			return DuplicateEventErr
		}

		if conf.Available == 0 {
			conf.Available = 5
		}

		if conf.Capacity == 0 {
			conf.Capacity = 10
		}

		if conf.FinishExpected.Before(time.Now()) || conf.FinishExpected.Format(displayTimeFormat) == "" || isInvalidDate(*conf.FinishExpected) {
			expectedFinishTime := now.AddDate(0, 0, 15)
			conf.FinishExpected = &expectedFinishTime
		}

		ev, err := d.ehost.CreateEventFromConfig(ctx, conf, d.conf.Rechaptcha)
		if err != nil {
			log.Error().Err(err).Msg("Error creating event from database event")
			return err
		}

		d.startEvent(ev)
	}
	return nil
}

func (d *daemon) bookEvent(ctx context.Context, req *pb.CreateEventRequest) error {
	user, err := getUserFromIncomingContext(ctx)
	if err != nil {
		return fmt.Errorf("invalid or no user information from incoming request %v", err)
	}
	sT, err := time.Parse(dbTimeFormat, req.StartTime)
	if err != nil {
		return fmt.Errorf("start time parsing error %v", err)
	}
	fT, err := time.Parse(dbTimeFormat, req.FinishTime)
	if err != nil {
		return fmt.Errorf("finish time parsing error %v", err)
	}
	// todo: will be updated
	isFree, err := d.isFree(sT, fT, req.Capacity)
	if err != nil {
		return err
	}
	isQuotaAvailable, err := d.checkUserQuota(ctx, user.Username)
	if err != nil {
		return err
	}
	if (!user.NPUser && isFree) || (user.NPUser && isQuotaAvailable && isFree) {
		log.Info().Str("Event Name ", req.Name).
			Str("Event  Tag", req.Tag).
			Msgf("Event is adding to database as booked ")
		_, err := d.dbClient.AddEvent(ctx, &pbc.AddEventRequest{
			Name: req.Name,
			Tag:  req.Tag,
			// risky getting value from static index
			Frontends:          req.Frontends[0],
			Exercises:          strings.Join(req.Exercises, ","),
			Available:          req.Available,
			Capacity:           req.Capacity,
			StartTime:          req.StartTime,
			ExpectedFinishTime: req.FinishTime,
			Status:             Booked,
			CreatedBy:          user.Username,
		})
		if err != nil {
			log.Warn().Msgf("problem for inserting booked event into table, err %v", err)
			return err
		}
	}
	log.Info().Msgf("Event %s is booked by %s  between %s and %s ", req.Tag, user.Name, req.StartTime, req.FinishTime)
	return nil
}

func (d *daemon) ListEventTeams(ctx context.Context, req *pb.ListEventTeamsRequest) (*pb.ListEventTeamsResponse, error) {
	var eventTeams []*pb.ListEventTeamsResponse_Teams
	evtag, err := store.NewTag(req.Tag)
	if err != nil {
		return nil, err
	}
	ev, err := d.eventPool.GetEvent(evtag)
	if err != nil {
		return nil, err
	}

	teams := ev.GetTeams()

	for _, t := range teams {

		accesedTime := t.LastAccessTime()

		eventTeams = append(eventTeams, &pb.ListEventTeamsResponse_Teams{
			Id:         strings.TrimSpace(t.ID()),
			Name:       strings.TrimSpace(t.Name()),
			Email:      strings.TrimSpace(t.Email()),
			AccessedAt: accesedTime.Format(displayTimeFormat),
		})

	}

	return &pb.ListEventTeamsResponse{Teams: eventTeams}, nil
}

func (d *daemon) StopEvent(req *pb.StopEventRequest, resp pb.Daemon_StopEventServer) error {
	ctx := resp.Context()
	var ev guacamole.Event
	log.Ctx(ctx).
		Info().
		Str("tag", req.Tag).
		Msg("stop event")

	user, err := getUserFromIncomingContext(ctx)
	if err != nil {
		log.Warn().Msgf("User credentials not found ! %v  ", err)
		return fmt.Errorf("user credentials could not found on context %v", err)
	}

	evtag, err := store.NewTag(req.Tag)
	if err != nil {
		return err
	}

	// unix time will be unique
	currentTime := strconv.Itoa(int(time.Now().Unix()))
	newEventTag := fmt.Sprintf("%s-%s", string(evtag), currentTime)
	// check event status from database
	// based on status take necessary action
	status, err := d.dbClient.GetEventStatus(ctx, &pbc.GetEventStatusRequest{EventTag: string(evtag)})
	if err != nil {
		return fmt.Errorf("error happened on getting status of event, err: %v", err)
	}

	if status.Status == Running || status.Status == Suspended {

		// retrieve tag of event from event pool
		ev, err = d.eventPool.GetEvent(evtag)
		if err != nil {
			return err
		}
		createdBy := ev.GetConfig().CreatedBy

		if (user.NPUser && user.Username == createdBy) || !user.NPUser {
			_, err := d.dbClient.SetEventStatus(ctx, &pbc.SetEventStatusRequest{EventTag: string(evtag), Status: Closed})
			if err != nil {
				return fmt.Errorf("error happened on setting up status of event, err: %v", err)
			}

			// tag of the event is removed from eventPool
			if err := d.eventPool.RemoveEvent(evtag); err != nil {
				return err
			}
			if err := ev.Close(); err != nil {
				return err
			}
			ev.Finish(newEventTag) // Finishing and archiving event....
			return nil
		} else {
			return fmt.Errorf("no priviledge to stop event %s", evtag)
		}
	}
	if status.Status == Booked {
		// no need to store booked event information
		// if it is deleted when its status booked
		if user.NPUser {
			eventResp, err := d.dbClient.GetEventByUser(ctx, &pbc.GetEventByUserReq{User: user.Username, Status: Booked})
			if err != nil {
				return fmt.Errorf("error on getting booked events for user %s, err : %v", user.Username, err)
			}
			for _, e := range eventResp.Events {
				if e.CreatedBy == user.Username {
					if err := d.dropEvent(ctx, req.Tag); err != nil {
						return fmt.Errorf("err : %v username : %s", err, user.Username)
					}
				}
			}
			return nil
		}
		if err := d.dropEvent(ctx, req.Tag); err != nil {
			return fmt.Errorf("err : %v username : %s", err, user.Username)
		}
	}
	return nil
}

func (d *daemon) ListEvents(ctx context.Context, req *pb.ListEventsRequest) (*pb.ListEventsResponse, error) {
	var events []*pb.ListEventsResponse_Events
	// in list events there is no need to distinguish based on users.
	// could be changed based on feedback
	// events are listed through database instead of eventPool

	eventsFromDB, err := d.dbClient.GetEvents(ctx, &pbc.GetEventRequest{Status: req.Status})
	if err != nil {
		log.Error().Msgf("Retrieving events from db in ListEvent function %v", err)
		return &pb.ListEventsResponse{}, err
	}

	for _, e := range eventsFromDB.Events {
		teamsFromDB, err := d.dbClient.GetEventTeams(ctx, &pbc.GetEventTeamsRequest{EventTag: e.Tag})
		if err != nil {
			log.Error().Msgf("Retrieving teams from db in ListEvent function %v", err)
			return &pb.ListEventsResponse{}, err
		}
		teamCount := int32(len(teamsFromDB.Teams))

		events = append(events, &pb.ListEventsResponse_Events{

			Tag:          string(e.Tag),
			Name:         e.Name,
			TeamCount:    teamCount,
			Exercises:    e.Exercises,
			Capacity:     e.Capacity,
			CreationTime: e.StartedAt,
			FinishTime:   e.ExpectedFinishTime, //This is the Expected finish time
			Status:       e.Status,
			CreatedBy:    e.CreatedBy,
		})
	}

	return &pb.ListEventsResponse{Events: events}, nil
}

func (d *daemon) createEventFromEventDB(ctx context.Context, conf store.EventConfig) error {

	if err := conf.Validate(); err != nil {
		return err
	}

	ev, err := d.ehost.CreateEventFromEventDB(ctx, conf, d.conf.Rechaptcha)
	if err != nil {
		log.Error().Err(err).Msg("Error creating event from database event")
		return err
	}

	d.startEvent(ev)

	return nil
}

// SuspendEvent manages suspension and resuming of given event
// according to isSuspend value from request, resume or suspend function will be called
func (d *daemon) SuspendEvent(req *pb.SuspendEventRequest, resp pb.Daemon_SuspendEventServer) error {
	ctx := resp.Context()
	user, err := getUserFromIncomingContext(ctx)
	if err != nil {
		log.Warn().Msgf("User credentials not found ! %v  ", err)
		return fmt.Errorf("user credentials could not found on context %v", err)
	}

	eventTag := store.Tag(req.EventTag)
	isSuspend := req.Suspend
	event, err := d.eventPool.GetEvent(eventTag)
	if err != nil {
		//return nil, err
		return err
	}
	createdBy := event.GetConfig().CreatedBy

	if !user.NPUser || (user.NPUser && user.Username == createdBy) {
		if isSuspend {
			if err := event.Suspend(ctx); err != nil {
				//return &pb.EventStatus{Status: "error", Entity: err.Error()}, err
				return err
			}
			event.SetStatus(Suspended)
			d.eventPool.handlers[eventTag] = suspendEventHandler()
			return nil
		}
		// it is important to get context from resp.Context
		// otherwise it cannot let it resume or suspend
		if err := event.Resume(ctx); err != nil {
			return err
		}
		d.eventPool.handlers[eventTag] = event.Handler()
		event.SetStatus(Running)
	}
	return nil
}

//removeDuplicates removes duplicated values in given list
// used incoming CreateEventRequest
func removeDuplicates(exercises []string) []string {
	k := make(map[string]bool)
	var uniqueExercises []string

	for _, e := range exercises {
		if _, v := k[e]; !v {
			k[e] = true
			uniqueExercises = append(uniqueExercises, e)
		}
	}
	return uniqueExercises
}

// a method which will start booked events by checking them in some predefined time intervals
func (d *daemon) visitBookedEvents() error {
	ctx := context.Background()
	now := time.Now()
	eventResponse, err := d.dbClient.GetEvents(context.Background(), &pbc.GetEventRequest{Status: Booked})
	if err != nil {
		log.Warn().Msgf("checking booked events error %v", err)
		return err
	}
	for _, event := range eventResponse.Events {
		requestedStartTime, _ := time.Parse(displayTimeFormat, event.StartedAt)
		if requestedStartTime.Before(now) || requestedStartTime.Equal(now) {
			// set status to running if booked event startTime passed.
			eventConfig := d.generateEventConfig(event, Running)
			if err := d.createEventFromEventDB(ctx, eventConfig); err != nil {
				log.Warn().Msgf("Error on creating booked event, event %s err %v", event.Tag, err)
				return fmt.Errorf("error on booked event creation %v", err)
			}
			// update status of event on database
			status, err := d.dbClient.SetEventStatus(ctx, &pbc.SetEventStatusRequest{Status: Running, EventTag: string(eventConfig.Tag)})
			if err != nil {
				return fmt.Errorf("status update failed err: %v", err)
			}
			// status.Status is state of the set event
			log.Info().Msgf("Status of event %s is updated with %d ", eventConfig.Tag, status.Status)
		}
	}
	return nil
}

func (d *daemon) generateEventConfig(event *pbc.GetEventResponse_Events, status int32) store.EventConfig {

	var instanceConfig []store.InstanceConfig
	var exercises []store.Tag

	requestedStartTime, _ := time.Parse(displayTimeFormat, event.StartedAt)
	requestedFinishTime, _ := time.Parse(displayTimeFormat, event.ExpectedFinishTime)
	listOfExercises := strings.Split(event.Exercises, ",")
	instanceConfig = append(instanceConfig, d.frontends.GetFrontends(event.Frontends)[0])
	for _, e := range listOfExercises {
		exercises = append(exercises, store.Tag(e))
	}

	eventConfig := store.EventConfig{
		Name:      event.Name,
		Tag:       store.Tag(event.Tag),
		Available: int(event.Available),
		Capacity:  int(event.Capacity),
		Lab: store.Lab{
			Frontends: instanceConfig,
			Exercises: exercises,
		},
		StartedAt:      &requestedStartTime,
		FinishExpected: &requestedFinishTime,
		FinishedAt:     nil,
		Status:         status,
		CreatedBy:      event.CreatedBy,
	}

	return eventConfig
}

// CloseEvents will fetch Running events from DB
// compares finish time, closes events if required.
func (d *daemon) closeEvents() error {
	ctx := context.Background()
	events, err := d.dbClient.GetEvents(ctx, &pbc.GetEventRequest{Status: Running})
	if err != nil {
		log.Warn().Msgf("get events error on close overdue events %v ", err)
		return err
	}

	for _, e := range events.Events {
		eTag := store.Tag(e.Tag)

		if isDelayed(e.ExpectedFinishTime) {
			currentTime := strconv.Itoa(int(time.Now().Unix()))
			newEventTag := fmt.Sprintf("%s-%s", e.Tag, currentTime)
			event, err := d.eventPool.GetEvent(eTag)
			if err != nil {
				log.Warn().Msgf("event pool get event error %v ", err)
				return err
			}
			_, err = d.dbClient.SetEventStatus(ctx, &pbc.SetEventStatusRequest{
				EventTag: string(eTag),
				Status:   Closed})
			if err != nil {
				log.Warn().Msgf("Error in setting up status of event in database side event: %s", string(eTag))
				return err
			}
			log.Debug().Msgf("Status is set to %d for event:  %s", Closed, string(eTag))
			if err := d.eventPool.RemoveEvent(eTag); err != nil {
				return err
			}
			if err := event.Close(); err != nil {
				return err
			}
			event.Finish(newEventTag)
		}
	}
	return nil
}

func isDelayed(customTime string) bool {
	now := time.Now()
	givenTime, _ := time.Parse(displayTimeFormat, customTime)
	return givenTime.Equal(now) || givenTime.Before(now)
}

func isInvalidDate(t time.Time) bool {
	if t == time.Date(0001, 01, 01, 00, 00, 00, 0000, time.UTC) {
		//log.Println("Error in parsing; invalid date 0001-01-01 00:00:00 +0000 UTC  ")
		return true
	}
	return false
}

func (d *daemon) checkUserQuota(ctx context.Context, user string) (bool, error) {
	usr, err := getUserFromIncomingContext(ctx)
	if err != nil {
		return false, fmt.Errorf("no user information err: %v", err)
	}
	var cost int32
	// will return invert case, if closed supplied to Status,
	// then it means that all events which are not closed
	events, err := d.dbClient.GetEventByUser(ctx, &pbc.GetEventByUserReq{User: user, Status: Closed})
	if err != nil {
		return false, err
	}
	for _, ev := range events.Events {
		cost += ev.Capacity
	}

	if usr.NPUser && cost > int32(NPUserMaxLabs) {
		return false, fmt.Errorf("user %s has %d labs already, out of quota error ", user, cost)
	}
	return true, nil

}
func (d *daemon) dropEvent(ctx context.Context, evTag string) error {
	r, err := d.dbClient.DropEvent(ctx, &pbc.DropEventReq{Status: Booked, Tag: evTag})
	if err != nil {
		return err
	}
	if r.IsDropped {
		log.Info().Msgf("Booked event %s is dropped at %s", evTag, time.Now().Format(dbTimeFormat))
	}
	return nil
}
