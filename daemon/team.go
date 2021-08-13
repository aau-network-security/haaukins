package daemon

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	pb "github.com/aau-network-security/haaukins/daemon/proto"
	"github.com/aau-network-security/haaukins/lab"
	"github.com/aau-network-security/haaukins/store"
	"github.com/aau-network-security/haaukins/svcs/guacamole"
	"github.com/aau-network-security/haaukins/virtual"
	"github.com/rs/zerolog/log"
)

const (
	INACTIVITY_DURATION = 8 // in hours
)

var (
	NoFlagMngtPrivErr   = errors.New("No privilege to see/solve challenges on an event created by others !")
	LabIsNotAssignedErr = errors.New("Lab is not assigned yet ! ")
	NoPrivToUpdate      = errors.New("No privilege to change team password !")
)

func (d *daemon) GetTeamInfo(ctx context.Context, in *pb.GetTeamInfoRequest) (*pb.GetTeamInfoResponse, error) {
	t, err := store.NewTag(in.EventTag)
	if err != nil {
		return nil, err
	}
	ev, err := d.eventPool.GetEvent(t)
	if err != nil {
		return nil, err
	}
	lab, ok := ev.GetLabByTeam(in.TeamId)
	if !ok {
		return nil, UnknownTeamErr
	}

	var instances []*pb.GetTeamInfoResponse_Instance
	for _, i := range lab.InstanceInfo() {
		instance := &pb.GetTeamInfoResponse_Instance{
			Image: i.Image,
			Type:  i.Type,
			Id:    i.Id,
			State: int32(i.State),
		}
		instances = append(instances, instance)
	}
	return &pb.GetTeamInfoResponse{Instances: instances}, nil

}

func (d *daemon) SetTeamSuspend(ctx context.Context, in *pb.SetTeamSuspendRequest) (*pb.Empty, error) {
	log.Ctx(ctx).Info().Str("team", in.TeamId).Msg("suspending team")

	// Extract lab for team
	t, err := store.NewTag(in.EventTag)
	if err != nil {
		return nil, err
	}
	ev, err := d.eventPool.GetEvent(t)
	if err != nil {
		return nil, err
	}
	lab, ok := ev.GetLabByTeam(in.TeamId)
	if !ok {
		return nil, UnknownTeamErr
	}

	// Suspend or wake the lab
	if in.Suspend {
		err = lab.Suspend(ctx)
	} else {
		err = lab.Resume(ctx)
	}
	return &pb.Empty{}, err
}

func (d *daemon) RestartTeamLab(req *pb.RestartTeamLabRequest, resp pb.Daemon_RestartTeamLabServer) error {
	log.Ctx(resp.Context()).
		Info().
		Str("event", req.EventTag).
		Str("lab", req.TeamId).
		Msg("restart lab")

	evtag, err := store.NewTag(req.EventTag)
	if err != nil {
		return err
	}

	ev, err := d.eventPool.GetEvent(evtag)
	if err != nil {
		return err
	}

	lab, ok := ev.GetLabByTeam(req.TeamId)
	if !ok {
		log.Warn().Msgf("Lab could not retrieved for team id %s ", req.TeamId)
		return NoLabByTeamIdErr
	}
	resp.Send(&pb.EventStatus{Status: fmt.Sprintf("Lab restarting for team [ %s ] is under process... ", req.TeamId)})

	if err := lab.Restart(resp.Context()); err != nil {
		return err
	}

	return nil
}

func (d *daemon) SolveChallenge(ctx context.Context, req *pb.SolveChallengeRequest) (*pb.SolveChallengeResponse, error) {
	var challenge store.Challenge
	user, err := getUserFromIncomingContext(ctx)
	if err != nil {
		log.Warn().Msgf("User credentials not found ! %v  ", err)
		return &pb.SolveChallengeResponse{}, fmt.Errorf("user credentials could not found on context %v", err)
	}
	event := d.eventPool.events[store.Tag(req.EventTag)]

	if user.NPUser && event.GetConfig().CreatedBy != user.Username {
		return &pb.SolveChallengeResponse{}, NoFlagMngtPrivErr
	}

	lab, ok := event.GetLabByTeam(req.TeamID)
	if !ok {
		return &pb.SolveChallengeResponse{}, LabIsNotAssignedErr
	}
	chals := lab.Environment().Challenges()

	for _, ch := range chals {
		if string(ch.Tag) == req.ChallengeTag {
			challenge = ch
			break
		}
	}

	flag := strings.TrimSpace(challenge.Value)

	for _, team := range event.GetTeams() {
		if team.ID() == req.TeamID {
			if err := team.VerifyFlag(challenge, flag); err != nil {
				return &pb.SolveChallengeResponse{}, err
			}
			break
		}
	}
	return &pb.SolveChallengeResponse{Status: fmt.Sprintf("Challenge [ %s ] solved on event [ %s ] for team [ %s ] !", challenge.Name, req.EventTag, req.TeamID)}, nil
}

func (d *daemon) GetTeamChals(ctx context.Context, req *pb.GetTeamInfoRequest) (*pb.TeamChalsInfo, error) {
	var flags []*pb.Flag
	event := d.eventPool.events[store.Tag(req.EventTag)]
	user, err := getUserFromIncomingContext(ctx)
	if err != nil {
		log.Warn().Msgf("User credentials not found ! %v  ", err)
		return &pb.TeamChalsInfo{}, fmt.Errorf("user credentials could not found on context %v", err)
	}

	if user.NPUser && event.GetConfig().CreatedBy != user.Username {
		return &pb.TeamChalsInfo{}, NoFlagMngtPrivErr
	}
	lab, ok := event.GetLabByTeam(req.TeamId)
	if !ok {
		return &pb.TeamChalsInfo{}, fmt.Errorf("Lab is not assigned yet ! ")
	}
	chals := lab.Environment().Challenges()

	for _, ch := range chals {
		flags = append(flags, &pb.Flag{
			ChallengeName: ch.Name,
			ChallengeTag:  string(ch.Tag),
			ChallengeFlag: ch.Value,
		})
	}
	return &pb.TeamChalsInfo{Flags: flags}, nil
}

func (d *daemon) UpdateTeamPassword(ctx context.Context, req *pb.UpdateTeamPassRequest) (*pb.UpdateTeamPassResponse, error) {
	usr, err := getUserFromIncomingContext(ctx)
	if err != nil {
		return &pb.UpdateTeamPassResponse{}, err
	}
	ev, ok := d.eventPool.events[store.Tag(req.EventTag)]
	if !ok {
		return &pb.UpdateTeamPassResponse{}, fmt.Errorf("Event [ %s ] could not be found ", req.EventTag)
	}
	if usr.NPUser && ev.GetConfig().CreatedBy != usr.Username {
		return &pb.UpdateTeamPassResponse{}, NoFlagMngtPrivErr
	}
	status, err := ev.UpdateTeamPassword(req.TeamID, req.Password, req.PasswordRepeat)
	if err != nil {
		return &pb.UpdateTeamPassResponse{}, err
	}

	return &pb.UpdateTeamPassResponse{Status: status}, nil
}

func (d *daemon) DeleteTeam(req *pb.DeleteTeamRequest, srv pb.Daemon_DeleteTeamServer) error {

	var waitGroup sync.WaitGroup
	usr, err := getUserFromIncomingContext(srv.Context())
	if err != nil {
		return err
	}
	ev, ok := d.eventPool.events[store.Tag(req.EvTag)]
	if !ok {
		return fmt.Errorf("Event [ %s ] could not be found ", req.EvTag)
	}
	if !usr.SuperUser && ev.GetConfig().CreatedBy != usr.Username {
		return fmt.Errorf("No privileges to delete team ")
	}
	srv.Send(&pb.DeleteTeamResponse{Message: fmt.Sprintf("Team deletion / Lab release and restart for team [ %s ] is under process... ", req.TeamId)})
	lh := ev.GetHub()
	tLab, ok := ev.GetLabByTeam(req.TeamId)
	if !ok {
		return fmt.Errorf("lab could not be found for team: [ %s ] on event [ %s ]", req.TeamId, req.EvTag)
	}
	var restartErr error
	waitGroup.Add(1)
	go func() {
		defer waitGroup.Done()
		if err := tLab.Restart(srv.Context()); err != nil {
			log.Debug().Msgf("Error on lab restart on de-assigning for team [ %s ] on event [ %s ]", req.TeamId, req.EvTag)
			restartErr = err
		}
	}()
	waitGroup.Wait()

	if restartErr != nil {
		return restartErr
	}

	lb := make(chan lab.Lab)
	sendLb := func() {
		lb <- tLab
	}

	go lh.Update(lb)
	go sendLb()

	_, err = ev.DeleteTeam(req.TeamId)
	if err != nil {
		return err
	}

	return nil
}

func checkTeamLab(ch chan guacamole.Event, wg *sync.WaitGroup) {
	now := time.Now().UTC()
	defer wg.Done()
	suspendLab := func(ev guacamole.Event) {
		for _, t := range ev.GetTeams() {
			difference := math.Round(now.Sub(t.LastAccessTime().UTC()).Hours()) // get in rounded format in hours
			if difference > INACTIVITY_DURATION {
				lab, ok := ev.GetLabByTeam(t.ID())
				if !ok {
					log.Error().Str("Team ID", t.ID()).
						Str("Team Name", t.Name()).
						Msgf("Error on team lab suspend/GetLabByTeam")
				}
				log.Info().Msgf("Suspending resources for team %s", t.Name())
				// check if lab might be nil !
				if lab != nil {
					// check if it is already suspended or not.
					for _, instanceInfo := range lab.InstanceInfo() {
						if instanceInfo.State != virtual.Suspended {
							if err := lab.Suspend(context.Background()); err != nil {
								log.Error().Msgf("Error on team lab suspend: %v", err)
							}
						}
					}
				}
			}
		}
	}
	select {
	case ev := <-ch:
		suspendLab(ev)
	}
}

// suspendTeams function will check all teams in all events
// then suspend their resources if lastAccessTime is higher than INACTIVITY_DURATION
func (d *daemon) suspendTeams() error {
	var wg sync.WaitGroup
	log.Info().Msg("Suspend teams called ! ")
	events := d.eventPool.GetAllEvents()
	ch := make(chan guacamole.Event, len(events))
	for _, ev := range events {
		go processEvent(ev, ch)
		wg.Add(1)
		go checkTeamLab(ch, &wg)
	}
	wg.Wait()
	return nil
}

func processEvent(ev guacamole.Event, ch chan guacamole.Event) {
	ch <- ev
}
