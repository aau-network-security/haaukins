package daemon

import (
	"context"
	"math"
	"time"

	"github.com/aau-network-security/haaukins/virtual"

	pb "github.com/aau-network-security/haaukins/daemon/proto"
	"github.com/aau-network-security/haaukins/store"
	"github.com/rs/zerolog/log"
)

const (
	INACTIVITY_DURATION = 8 // in hours
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

	if err := lab.Restart(resp.Context()); err != nil {
		return err
	}

	return nil
}

// suspendTeams function will check all teams in all events
// then suspend their resources if lastAccessTime is higher than INACTIVITY_DURATION
func (d *daemon) suspendTeams() error {
	var suspendError error
	log.Info().Msg("Suspend teams called ! ")
	events := d.eventPool.GetAllEvents()
	now := time.Now()

	for _, e := range events {
		for _, t := range e.GetTeams() {
			if !t.LastAccessTime().IsZero() {
				difference := math.Round(now.Sub(t.LastAccessTime()).Minutes()) // get in rounded format in hours
				if difference > INACTIVITY_DURATION {
					lab, ok := e.GetLabByTeam(t.ID())
					if !ok {
						return UnknownTeamErr
					}
					// check if it is already suspended or not.
					lab.InstanceInfo()
					for _, instanceInfo := range lab.InstanceInfo() {
						if instanceInfo.State != virtual.Suspended {
							go func() {
								if err := lab.Suspend(context.Background()); err != nil {
									suspendError = err
								}
							}()
						}
						return suspendError
					}
				}
			}
		}
	}
	return nil
}
