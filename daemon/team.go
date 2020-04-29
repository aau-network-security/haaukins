package daemon

import (
	"context"
	pb "github.com/aau-network-security/haaukins/daemon/proto"
	"github.com/aau-network-security/haaukins/store"
	"github.com/rs/zerolog/log"
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