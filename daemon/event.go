package daemon

import (
	"context"
	"fmt"
	"strings"
	"time"

	pbc "github.com/aau-network-security/haaukins/store/proto"

	"github.com/aau-network-security/haaukins/svcs/guacamole"

	pb "github.com/aau-network-security/haaukins/daemon/proto"
	"github.com/aau-network-security/haaukins/store"
	"github.com/rs/zerolog/log"
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

	log.Ctx(resp.Context()).
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
	now := time.Now()
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

	finishTime, _ := time.Parse("2006-01-02", req.FinishTime)

	// handling booked events might be changed
	startTime, _ := time.Parse(time.RFC3339, req.StartTime)
	// difference  in days
	// if there is no difference in days it means event  will be
	// started immediately
	difference := int(startTime.Sub(now).Hours() / 24)
	if difference > 1 {
		if err := d.bookEvent(context.Background(), req); err != nil {
			return err
		}
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
		Status: Running,
	}

	if err := conf.Validate(); err != nil {
		return err
	}

	_, err := d.eventPool.GetEvent(evtag)

	if err == nil {
		return DuplicateEventErr
	}

	if conf.Available == 0 {
		conf.Available = 5
	}

	if conf.Capacity == 0 {
		conf.Capacity = 10
	}

	if conf.FinishExpected.Before(time.Now()) || conf.FinishExpected.String() == "" {
		expectedFinishTime := now.AddDate(0, 0, 15)
		conf.FinishExpected = &expectedFinishTime
	}

	loggerInstance := &GrpcLogger{resp: resp}
	ctx := context.WithValue(resp.Context(), "grpc_logger", loggerInstance)

	ev, err := d.ehost.CreateEventFromConfig(ctx, conf)
	if err != nil {
		return err
	}
	d.startEvent(ev)

	return nil
}

func (d *daemon) bookEvent(ctx context.Context, req *pb.CreateEventRequest) error {
	user, err := getUserFromIncomingContext(ctx)
	if err != nil {
		return fmt.Errorf("invalid or no user information from incoming request %v", err)
	}
	if !user.SuperUser && calculateTotalConsumption(d) < 80 {
		// todo: create db record as booked event
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
		})
		if err != nil {
			log.Warn().Msgf("problem for inserting booked event into table, err %v", err)
			return err
		}
	}
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
	log.Ctx(resp.Context()).
		Info().
		Str("tag", req.Tag).
		Msg("stop event")

	evtag, err := store.NewTag(req.Tag)
	if err != nil {
		return err
	}
	// retrieve tag of event from event pool
	ev, err := d.eventPool.GetEvent(evtag)
	if err != nil {
		return err
	}
	// tag of the event is removed from eventPool
	if err := d.eventPool.RemoveEvent(evtag); err != nil {
		return err
	}

	ev.Close()
	ev.Finish() // Finishing and archiving event....
	return nil
}

func (d *daemon) ListEvents(ctx context.Context, req *pb.ListEventsRequest) (*pb.ListEventsResponse, error) {
	var events []*pb.ListEventsResponse_Events

	for _, event := range d.eventPool.GetAllEvents() {
		conf := event.GetConfig()

		var exercises []string
		for _, ex := range conf.Lab.Exercises {
			exercises = append(exercises, string(ex))
		}

		events = append(events, &pb.ListEventsResponse_Events{

			Tag:          string(conf.Tag),
			Name:         conf.Name,
			TeamCount:    int32(len(event.GetTeams())),
			Exercises:    strings.Join(exercises, ","),
			Capacity:     int32(conf.Capacity),
			CreationTime: conf.StartedAt.Format(displayTimeFormat),
			FinishTime:   conf.FinishExpected.Format(displayTimeFormat), //This is the Expected finish time
			Status:       conf.Status,
		})
	}

	return &pb.ListEventsResponse{Events: events}, nil
}

func (d *daemon) createEventFromEventDB(ctx context.Context, conf store.EventConfig) error {

	if err := conf.Validate(); err != nil {
		return err
	}

	ev, err := d.ehost.CreateEventFromEventDB(ctx, conf)
	if err != nil {
		log.Error().Err(err).Msg("Error creating event from database event")
		return err
	}

	d.startEvent(ev)

	return nil
}

// SuspendEvent manages suspension and resuming of given event
// according to isSuspend value from request, resume or suspend function will be called
func (d *daemon) SuspendEvent(req *pb.SuspendEventRequest, server pb.Daemon_SuspendEventServer) error {
	eventTag := store.Tag(req.EventTag)
	isSuspend := req.Suspend
	event, err := d.eventPool.GetEvent(eventTag)
	if err != nil {
		//return nil, err
		return err
	}
	if isSuspend {
		if err := event.Suspend(context.Background()); err != nil {
			//return &pb.EventStatus{Status: "error", Entity: err.Error()}, err
			return err
		}
		event.SetStatus(Suspended)
		d.eventPool.handlers[eventTag] = suspendEventHandler()
		return nil
	}

	if err := event.Resume(context.Background()); err != nil {
		return err
	}
	d.eventPool.handlers[eventTag] = event.Handler()
	event.SetStatus(Running)
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
	var instanceConfig []store.InstanceConfig
	var exercises []store.Tag

	// code quality is NOT optimum :\

	for _, event := range eventResponse.Events {
		requestedStartTime, _ := time.Parse(time.RFC3339, event.StartedAt)
		if requestedStartTime.Before(now) || requestedStartTime.Equal(now) {
			listOfExercises := strings.Split(event.Exercises, ",")
			instanceConfig = append(instanceConfig, d.frontends.GetFrontends(event.Frontends)[0])
			for _, e := range listOfExercises {
				exercises = append(exercises, store.Tag(e))
			}
			event := store.EventConfig{
				Name:      event.Name,
				Tag:       store.Tag(event.Tag),
				Available: int(event.Available),
				Capacity:  int(event.Capacity),
				Lab: store.Lab{
					Frontends: instanceConfig,
					Exercises: exercises,
				},
				StartedAt:      &requestedStartTime,
				FinishExpected: nil,
				FinishedAt:     nil,
				Status:         Running,
			}
			if err := d.createEventFromEventDB(ctx, event); err != nil {
				log.Warn().Msgf("Error on creating booked event, event %s err %v", event.Tag, err)
				return fmt.Errorf("error on booked event creation %v", err)
			}
		}
	}
	return nil
}
