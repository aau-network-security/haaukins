package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aau-network-security/haaukins/virtual/vbox"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	pb "github.com/aau-network-security/haaukins/daemon/proto"
	eproto "github.com/aau-network-security/haaukins/exercise/ex-proto"
	"github.com/aau-network-security/haaukins/store"
	pbc "github.com/aau-network-security/haaukins/store/proto"
	"github.com/aau-network-security/haaukins/svcs/guacamole"
	"github.com/rs/zerolog/log"
)

const (
	charSet = "abcdefghijklmnopqrstuvwxyz" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	numSet     = "0123456789"
	NoVPN      = 0
	VPN        = 1
	VPNBrowser = 2
)

var (
	NoPrivilegeToStressTest = errors.New("No privilege to have stress test on Haaukins !")
	NPUserMaxLabs           = 40
	NotAvailableTag         = "not available tag, there is already an event which is either running, booked or suspended"
	vpnIPPools              = newIPPoolFromHost()
	CapacityExceedsErr      = errors.New("VPN Events can have maximum 252 people on board !")
	OutOfQuota              = errors.New("Out of quota for members, you have limited access")
)

// INITIAL POINT OF CREATE EVENT FUNCTION, IT INITIALIZE EVENT AND ADDS EVENTPOOL
func (d *daemon) startEvent(ev guacamole.Event) {
	conf := ev.GetConfig()
	var frontendNames []string
	if ev.GetConfig().OnlyVPN != VPN {
		for _, f := range conf.Lab.Frontends {
			frontendNames = append(frontendNames, f.Image)
		}
	}

	log.Info().
		Str("Name", conf.Name).
		Str("Tag", string(conf.Tag)).
		Str("SecretKey", conf.SecretKey).
		Int("Available", conf.Available).
		Int("Capacity", conf.Capacity).
		Strs("Frontends", frontendNames).
		Msg("Creating event")

	go ev.Start(context.TODO())

	d.eventPool.AddEvent(ev)
}

func (d *daemon) CreateEvent(req *pb.CreateEventRequest, resp pb.Daemon_CreateEventServer) error {

	loggerInstance := &GrpcLogger{resp: resp}
	vpnAddress := ""
	ctx := context.WithValue(resp.Context(), "grpc_logger", loggerInstance)
	log.Ctx(ctx).
		Info().
		Str("tag", req.Tag).
		Str("name", req.Name).
		Int32("available", req.Available).
		Int32("capacity", req.Capacity).
		Strs("frontends", req.Frontends).
		Strs("exercises", req.Exercises).
		Strs("disabled exercises", req.DisableExercises).
		Str("startTime", req.StartTime).
		Str("finishTime", req.FinishTime).
		Str("SecretKey", req.SecretEvent).
		Int32("VPN", req.OnlyVPN).
		Msg("create event")
	// get random subnet for vpn connection
	// check from database if subnet is already assigned to an event or not
	user, err := getUserFromIncomingContext(ctx)
	if err != nil {
		log.Warn().Msgf("User credentials not found ! %v  ", err)
		return fmt.Errorf("user credentials could not found on context %v", err)
	}

	if (req.OnlyVPN == VPN || req.OnlyVPN == VPNBrowser) && req.Capacity > 253 {
		resp.Send(&pb.LabStatus{ErrorMessage: CapacityExceedsErr.Error()})
		return CapacityExceedsErr
	}

	if user.NPUser && req.Capacity > int32(NPUserMaxLabs) {
		resp.Send(&pb.LabStatus{ErrorMessage: OutOfQuota.Error()})
		return OutOfQuota
	}

	isEligible, err := d.checkUserQuota(ctx, user.Username)
	if err != nil {
		resp.Send(&pb.LabStatus{ErrorMessage: err.Error()})
		return err
	}

	if (user.NPUser && isEligible) || !user.NPUser {

		now := time.Now()
		if ReservedSubDomains[strings.ToLower(req.Tag)] {
			resp.Send(&pb.LabStatus{ErrorMessage: ReservedDomainErr.Error()})
			return ReservedDomainErr
		}

		uniqueExercisesList := removeDuplicates(req.Exercises)

		disabledExs := convertToTags(req.DisableExercises)

		tags := make([]store.Tag, len(uniqueExercisesList))
		exs, tagErr := d.exClient.GetExerciseByTags(ctx, &eproto.GetExerciseByTagsRequest{Tag: uniqueExercisesList})
		if tagErr != nil {
			resp.Send(&pb.LabStatus{ErrorMessage: tagErr.Error()})
			return tagErr
		}

		for i, s := range exs.Exercises {
			if s.Secret && !user.SuperUser {
				return fmt.Errorf("No priviledge to create event with secret challenges [ %s ]. Secret challenges unique to super users only.", s.Tag)
			}
			t, err := store.NewTag(s.Tag)
			if err != nil {
				return err
			}
			tags[i] = t
		}
		evtag, _ := store.NewTag(req.Tag)
		finishTime, _ := time.Parse(dbTimeFormat, req.FinishTime)
		startTime, _ := time.Parse(dbTimeFormat, req.StartTime)

		if isInvalidDate(startTime) {
			invalidDateErr := fmt.Errorf("invalid startTime format %v", startTime)
			resp.Send(&pb.LabStatus{ErrorMessage: invalidDateErr.Error()})
			return invalidDateErr
		}

		// checking through eventPool is not good enough since booked events are not added to eventpool
		isEventExist, err := d.dbClient.IsEventExists(ctx, &pbc.GetEventByTagReq{
			EventTag: req.Tag,
			// it will take INVERT condition which means that query from
			//Running, Suspended and Booked events
			Status: Closed,
		})
		if err != nil {
			noEventExist := fmt.Errorf("event does not exist or something is wrong: %v", err)
			resp.Send(&pb.LabStatus{ErrorMessage: noEventExist.Error()})
			return noEventExist
		}
		if isEventExist.IsExist {
			resp.Send(&pb.LabStatus{ErrorMessage: NotAvailableTag})
			return fmt.Errorf(NotAvailableTag)
		}
		log.Debug().Msgf("Checked existing events through database.")
		// difference  in days
		// if there is no difference in days it means event  will be
		// started immediately
		difference := math.Round(startTime.Sub(now).Hours() / 24)
		if difference >= 1 {
			if err := d.bookEvent(ctx, req); err != nil {
				resp.Send(&pb.LabStatus{ErrorMessage: err.Error()})
				return err
			}
			return nil
		}
		// 25.43
		if req.OnlyVPN == int32(VPN) || req.OnlyVPN == int32(VPNBrowser) {
			vpnIP, err := getVPNIP()
			if err != nil {
				log.Error().Msgf("Error on getting IP for VPN connection error: %v", err)
			}
			vpnAddress = fmt.Sprintf("%s.240.1/22", vpnIP)
		}

		secretKey := strings.TrimSpace(req.SecretEvent)

		conf := store.EventConfig{
			Name:           req.Name,
			Tag:            evtag,
			Available:      int(req.Available),
			Capacity:       int(req.Capacity),
			Host:           d.conf.Host.Http,
			StartedAt:      &startTime,
			FinishExpected: &finishTime,
			Lab: store.Lab{
				Frontends:         d.frontends.GetFrontends(req.Frontends...),
				Exercises:         tags,
				DisabledExercises: disabledExs,
			},
			Status:     Running,
			CreatedBy:  user.Username,
			OnlyVPN:    req.OnlyVPN, // 0 novpn 1 // vpn 2 // browser+vpn
			VPNAddress: vpnAddress,
			SecretKey:  secretKey,
		}

		if err := conf.Validate(); err != nil {
			return err
		}
		log.Debug().Str("event", string(conf.Tag)).
			Msgf("Event configuration is validated")
		_, err = d.eventPool.GetEvent(evtag)

		if err == nil {
			resp.Send(&pb.LabStatus{ErrorMessage: DuplicateEventErr.Error()})
			return DuplicateEventErr
		}

		if conf.Available == 0 {
			conf.Available = 2
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
			resp.Send(&pb.LabStatus{ErrorMessage: err.Error()})
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
			OnlyVPN:            req.OnlyVPN,
			SecretKey:          req.SecretEvent,
			DisabledExercises:  strings.Join(req.DisableExercises, ","),
		})
		if err != nil {
			log.Warn().Msgf("problem for inserting booked event into table, err %v", err)
			return err
		}
	}
	log.Info().Msgf("Event %s is booked by %s  between %s and %s ", req.Tag, user.Name, req.StartTime, req.FinishTime)
	return nil
}

func convertToTags(exs []string) []store.Tag {
	tags := make([]store.Tag, len(exs))
	for i, v := range exs {
		tags[i] = store.Tag(v)
	}
	return tags
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
		eventConfig := ev.GetConfig()
		createdBy := eventConfig.CreatedBy
		if eventConfig.OnlyVPN == VPN || eventConfig.OnlyVPN == VPNBrowser {
			vpnIP := strings.ReplaceAll(eventConfig.VPNAddress, ".240.1/22", "")
			vpnIPPools.ReleaseIP(vpnIP)
			log.Debug().Str("VPN IP", vpnIP).Msgf("VPN IP is released from allocated pool!")
		}

		if (user.NPUser && user.Username == createdBy) || !user.NPUser {
			// remove the corrosponding event folder
			if err := vbox.RemoveEventFolder(string(evtag)); err != nil {
				//do nothing
			}
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
	var event *pb.ListEventsResponse_Events
	// in list events there is no need to distinguish based on users.
	// could be changed based on feedback
	// events are listed through database instead of eventPool
	user, err := getUserFromIncomingContext(ctx)
	if err != nil {
		log.Warn().Msgf("User credentials not found ! %v  ", err)
		return &pb.ListEventsResponse{}, fmt.Errorf("user credentials could not found on context %v", err)
	}

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
		if user.SuperUser || user.Username == e.CreatedBy {
			event = &pb.ListEventsResponse_Events{

				Tag:          string(e.Tag),
				Name:         e.Name,
				TeamCount:    teamCount,
				Exercises:    e.Exercises,
				Availability: e.Available,
				Capacity:     e.Capacity,
				CreationTime: e.StartedAt,
				FinishTime:   e.ExpectedFinishTime, //This is the Expected finish time
				Status:       e.Status,
				CreatedBy:    e.CreatedBy,
				SecretEvent:  e.SecretKey,
			}
		} else {
			event = &pb.ListEventsResponse_Events{

				Tag:          string(e.Tag),
				Name:         e.Name,
				TeamCount:    teamCount,
				Exercises:    e.Exercises,
				Availability: e.Available,
				Capacity:     e.Capacity,
				CreationTime: e.StartedAt,
				FinishTime:   e.ExpectedFinishTime, //This is the Expected finish time
				Status:       e.Status,
				CreatedBy:    e.CreatedBy,
			}
		}
		events = append(events, event)
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

func (d *daemon) AddNotification(ctx context.Context, req *pb.AddNotificationRequest) (*pb.AddNotificationResponse, error) {
	usr, err := getUserFromIncomingContext(ctx)
	if err != nil {
		return &pb.AddNotificationResponse{}, err
	}
	if !usr.SuperUser {
		return &pb.AddNotificationResponse{}, errors.New("this feature is only available for super users ")
	}
	var waitGroup sync.WaitGroup
	message := strings.TrimSpace(req.Message)
	loggedInUsers := req.LoggedUsers

	for _, e := range d.eventPool.events {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			if err := e.AddNotification(message, loggedInUsers); err != nil {
				log.Error().Msgf("[add-notification] err: %v ", err)
				// todo: might be added return
			}
		}()
		waitGroup.Wait()
	}

	return &pb.AddNotificationResponse{Response: "Given notification set for all events "}, nil
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
	vpnAddress := ""
	eventResponse, err := d.dbClient.GetEvents(context.Background(), &pbc.GetEventRequest{Status: Booked})
	if err != nil {
		log.Warn().Msgf("checking booked events error %v", err)
		return err
	}
	for _, event := range eventResponse.Events {
		requestedStartTime, _ := time.Parse(displayTimeFormat, event.StartedAt)
		if requestedStartTime.Before(now) || requestedStartTime.Equal(now) {
			// set status to running if booked event startTime passed.

			if event.OnlyVPN == int32(VPN) || event.OnlyVPN == int32(VPNBrowser) {
				vpnIP, err := getVPNIP()
				if err != nil {
					log.Error().Msgf("Error on getting IP for VPN connection error: %v", err)
				}
				vpnAddress = fmt.Sprintf("%s.240.1/22", vpnIP)
			}

			eventConfig := d.generateEventConfig(event, Running, vpnAddress)
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

func (d *daemon) generateEventConfig(event *pbc.GetEventResponse_Events, status int32, vpnAddress string) store.EventConfig {

	var instanceConfig []store.InstanceConfig
	var exercises []store.Tag
	var disabledExerciseTags []store.Tag
	requestedStartTime, _ := time.Parse(displayTimeFormat, event.StartedAt)
	requestedFinishTime, _ := time.Parse(displayTimeFormat, event.ExpectedFinishTime)
	listOfExercises := strings.Split(event.Exercises, ",")
	disabledExs := strings.Split(event.DisabledExercises, ",")
	instanceConfig = append(instanceConfig, d.frontends.GetFrontends(event.Frontends)[0])
	for _, e := range listOfExercises {
		exercises = append(exercises, store.Tag(e))
	}
	for _, e := range disabledExs {
		disabledExerciseTags = append(disabledExerciseTags, store.Tag(e))
	}

	log.Debug().Str("Event name", event.Name).
		Str("Event tag", event.Tag).
		Int32("available", event.Available).
		Int32("capacity", event.Capacity).
		Str("frontend", event.Frontends).
		Str("exercises", event.Exercises).
		Str("disabled exercises", event.DisabledExercises).
		Str("startTime", event.StartedAt).
		Str("finishTime", event.ExpectedFinishTime).
		Str("SecretKey", event.SecretKey).
		Int32("VPN", event.OnlyVPN).Msgf("Generating event config from database !")

	eventConfig := store.EventConfig{
		Name:      event.Name,
		Host:      d.conf.Host.Http,
		Tag:       store.Tag(event.Tag),
		Available: int(event.Available),
		Capacity:  int(event.Capacity),
		Lab: store.Lab{
			Frontends:         instanceConfig,
			Exercises:         exercises,
			DisabledExercises: disabledExerciseTags,
		},
		StartedAt:      &requestedStartTime,
		FinishExpected: &requestedFinishTime,
		FinishedAt:     nil,
		Status:         status,
		CreatedBy:      event.CreatedBy,
		VPNAddress:     vpnAddress,
		OnlyVPN:        event.OnlyVPN, // 0 NoVPN 1 VPN 2 Browser+VPN
		SecretKey:      event.SecretKey,
	}

	return eventConfig
}

// CloseEvents closes overdue events
func (d *daemon) closeEvents() error {
	var wg sync.WaitGroup
	allEvents := d.eventPool.GetAllEvents()
	ch := make(chan guacamole.Event, len(allEvents))
	for _, ev := range allEvents {
		go processEvent(ev, ch)
		wg.Add(1)
		go d.closeEvent(ch, &wg)
	}
	wg.Wait()
	return nil
}

func (d *daemon) closeEvent(ch chan guacamole.Event, wg *sync.WaitGroup) error {
	log.Info().Msgf("Running close events...")
	ctx := context.Background()
	var closeErr error
	defer wg.Done()
	closer := func(ev guacamole.Event) {
		e := ev.GetConfig()
		log.Info().Msgf("Running close events, checking %s", e.Tag)
		if e.OnlyVPN == VPN || e.OnlyVPN == VPNBrowser {
			vpnIP := strings.ReplaceAll(e.VPNAddress, ".240.1/22", "")
			vpnIPPools.ReleaseIP(vpnIP)
			log.Debug().Str("VPN IP", vpnIP).Msgf("VPNIP is released from allocated pool!")
		}

		if isDelayed(e.FinishExpected.Format(displayTimeFormat)) {
			currentTime := strconv.Itoa(int(time.Now().Unix()))
			newEventTag := fmt.Sprintf("%s-%s", e.Tag, currentTime)
			event, err := d.eventPool.GetEvent(e.Tag)
			_ = vbox.RemoveEventFolder(string(e.Tag))
			if err != nil {
				log.Warn().Msgf("event pool get event error %v ", err)
				closeErr = err
			}
			_, err = d.dbClient.SetEventStatus(ctx, &pbc.SetEventStatusRequest{
				EventTag: string(e.Tag),
				Status:   Closed})
			if err != nil {
				log.Warn().Msgf("Error in setting up status of event in database side event: %s", string(e.Tag))
				closeErr = err
			}
			log.Debug().Msgf("Status is set to %d for event:  %s", Closed, string(e.Tag))
			if err := d.eventPool.RemoveEvent(e.Tag); err != nil {
				closeErr = err
			}
			if err := event.Close(); err != nil {
				closeErr = err
			}
			event.Finish(newEventTag)
		}
	}

	select {
	case ev := <-ch:
		closer(ev)
	}
	return closeErr
}

// StressEvent is making requests to daemon to see how daemon can handle them
func (d *daemon) StressEvent(ctx context.Context, req *pb.TestEventLoadReq) (*pb.TestEventLoadResp, error) {
	user, err := getUserFromIncomingContext(ctx)
	if err != nil {
		return &pb.TestEventLoadResp{}, err
	}
	if !user.SuperUser {
		return &pb.TestEventLoadResp{}, NoPrivilegeToStressTest
	}

	ev, err := d.eventPool.GetEvent(store.Tag(req.EventName))
	if ev == nil {
		return &pb.TestEventLoadResp{}, fmt.Errorf("no such an event called %s, error: %v !", req.EventName, err)
	}

	if ev.GetConfig().Capacity < int(req.NumberOfTeams) {
		return &pb.TestEventLoadResp{}, errors.New("event capacity is less than provided number of teams. skipping testing...")
	}
	var port, protocol string
	if d.conf.Certs.Enabled {
		port = strconv.FormatUint(uint64(d.conf.Port.Secure), 10)
		protocol = "https://"
	} else {
		port = strconv.FormatUint(uint64(d.conf.Port.InSecure), 10)
		protocol = "http://"
	}
	endPoint := fmt.Sprintf(protocol + req.EventName + "." + d.conf.Host.Http + ":" + port + "/signup")
	resp := make(chan string)
	for i := 0; i < int(req.NumberOfTeams); i++ {
		go func() {
			resp <- d.postRequest(endPoint, protocol)
		}()
	}
	response := <-resp

	return &pb.TestEventLoadResp{SignUpResult: response}, nil
}

// constructRandomValues will create random form for signing up
func constructRandomValues() url.Values {
	name := stringWithCharset(10, charSet)
	email := fmt.Sprintf("%s@%s.com", stringWithCharset(10, charSet), stringWithCharset(10, charSet))
	password := fmt.Sprintf("%s", stringWithCharset(6, numSet))
	form := url.Values{}
	form.Add("email", email)
	form.Add("team-name", name)
	form.Add("password", password)
	form.Add("password-repeat", password)
	return form
}

func (d *daemon) postRequest(endPoint, protocol string) string {
	hc := http.Client{}
	v := constructRandomValues()
	postReq, err := http.NewRequest("POST", endPoint, strings.NewReader(v.Encode()))
	if err != nil {
		return err.Error()
	}
	postReq.Form = v
	postReq.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 6.3; WOW64; rv:43.0) Gecko/20100101 Firefox/43.0")
	postReq.Header.Add("Referer", protocol+endPoint)
	postReq.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp, err := hc.Do(postReq)
	if err != nil {
		return err.Error()
	}
	return resp.Status
}

// StringWithCharset will return random characters
func stringWithCharset(length int, charset string) string {
	var seededRand *rand.Rand = rand.New(
		rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
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

func getVPNIP() (string, error) {
	// by default CreateEvent function will create event VPN  + Kali Connection
	ip, err := vpnIPPools.Get()
	if err != nil {
		return "", err
	}
	return ip, nil
}

func (d *daemon) SaveProfile(req *pb.SaveProfileRequest, resp pb.Daemon_SaveProfileServer) error {
	ctx := resp.Context()
	user, err := getUserFromIncomingContext(ctx)
	if err != nil {
		log.Warn().Msgf("User credentials not found ! %v  ", err)
		return fmt.Errorf("user credentials could not found on context %v", err)
	}
	log.Ctx(ctx).
		Info().
		Str("name", req.Name).
		Msg("Saving profile")
	log.Info().Str("profileName", req.Name).Msg("Trying to save profile")
	var challenges []*pbc.AddProfileRequest_Challenge
	for _, c := range req.Challenges {
		challenges = append(challenges, &pbc.AddProfileRequest_Challenge{
			Tag:  c.Tag,
			Name: c.Name,
		})
	}

	if !user.NPUser {
		resp, err := d.dbClient.AddProfile(ctx, &pbc.AddProfileRequest{
			Name:       req.Name,
			Secret:     req.Secret,
			Challenges: challenges,
		})
		if err != nil {
			return fmt.Errorf("Error when adding profile: %e", err)
		}
		return errors.New(resp.ErrorMessage)
	}

	return errors.New("You don't have the privilege to create profiles")

}

func (d *daemon) ListProfiles(ctx context.Context, req *pb.Empty) (*pb.ListProfilesResponse, error) {
	var profiles []*pb.ListProfilesResponse_Profile
	user, err := getUserFromIncomingContext(ctx)
	if err != nil {
		return &pb.ListProfilesResponse{}, NoUserInformation
	}

	profilesStore, err := d.dbClient.GetProfiles(ctx, &pbc.EmptyRequest{})
	if err != nil {
		return &pb.ListProfilesResponse{}, fmt.Errorf("[store-service]: ERR getting profiles %v", err)
	}
	var profs []store.Profile
	for _, p := range profilesStore.Profiles {
		profile, err := protobufToJson(p)
		if err != nil {
			return nil, err
		}
		pstruct := store.Profile{}
		json.Unmarshal([]byte(profile), &pstruct)
		profs = append(profs, pstruct)
	}
	for _, p := range profs {
		var chals []*pb.ListProfilesResponse_Profile_Challenge
		if !user.SuperUser && p.Secret {
			continue
		}
		for _, c := range p.Challenges {
			chals = append(chals, &pb.ListProfilesResponse_Profile_Challenge{
				Tag:  c.Tag,
				Name: c.Name,
			})
		}
		profiles = append(profiles, &pb.ListProfilesResponse_Profile{
			Name:       p.Name,
			Secret:     p.Secret,
			Challenges: chals,
		})
	}
	return &pb.ListProfilesResponse{Profiles: profiles}, nil
}

func (d *daemon) EditProfile(req *pb.SaveProfileRequest, resp pb.Daemon_EditProfileServer) error {
	ctx := resp.Context()
	user, err := getUserFromIncomingContext(ctx)
	if err != nil {
		log.Warn().Msgf("User credentials not found ! %v  ", err)
		return fmt.Errorf("user credentials could not found on context %v", err)
	}
	log.Ctx(ctx).
		Info().
		Str("name", req.Name).
		Msg("Updating profile")
	log.Info().Str("profileName", req.Name).Msg("Trying to update profile")
	var challenges []*pbc.UpdateProfileRequest_Challenge
	for _, c := range req.Challenges {
		challenges = append(challenges, &pbc.UpdateProfileRequest_Challenge{
			Tag:  c.Tag,
			Name: c.Name,
		})
	}

	if !user.NPUser {
		_, err := d.dbClient.UpdateProfile(ctx, &pbc.UpdateProfileRequest{
			Name:       req.Name,
			Secret:     req.Secret,
			Challenges: challenges,
		})
		if err != nil {
			return fmt.Errorf("Error when updating profile: %e", err)
		}
		return nil
	}

	return errors.New("You don't have the privilege to update profiles")
}

func (d *daemon) DeleteProfile(req *pb.DeleteProfileRequest, resp pb.Daemon_DeleteProfileServer) error {
	ctx := resp.Context()
	user, err := getUserFromIncomingContext(ctx)
	if err != nil {
		log.Warn().Msgf("User credentials not found ! %v  ", err)
		return fmt.Errorf("user credentials could not found on context %v", err)
	}
	log.Ctx(ctx).
		Info().
		Str("name", req.Name).
		Msg("Deleting profile")
	log.Info().Str("profileName", req.Name).Msg("Trying to delete profile")
	if !user.NPUser {
		_, err := d.dbClient.DeleteProfile(ctx, &pbc.DelProfileRequest{
			Name: req.Name,
		})
		if err != nil {
			return fmt.Errorf("Error when deleting profile: %e", err)
		}
		return nil
	}

	return errors.New("You don't have the privilege to delete profiles")
}
