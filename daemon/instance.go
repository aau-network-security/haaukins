package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pb "github.com/aau-network-security/haaukins/daemon/proto"
	"github.com/aau-network-security/haaukins/store"
	"github.com/rs/zerolog/log"
)

func (d *daemon) SetFrontendMemory(ctx context.Context, in *pb.SetFrontendMemoryRequest) (*pb.Empty, error) {
	err := d.frontends.SetMemoryMB(in.Image, uint(in.MemoryMB))
	return &pb.Empty{}, err
}

func (d *daemon) SetFrontendCpu(ctx context.Context, in *pb.SetFrontendCpuRequest) (*pb.Empty, error) {
	err := d.frontends.SetCpu(in.Image, float64(in.Cpu))
	return &pb.Empty{}, err
}

func (d *daemon) ResetFrontends(ctx context.Context, req *pb.ResetFrontendsRequest) (*pb.ResetTeamStatus, error) {
	log.Ctx(ctx).Info().
		Int("n-teams", len(req.Teams)).
		Msg("reset frontends")

	evtag, err := store.NewTag(req.EventTag)
	if err != nil {
		return nil, err
	}

	ev, err := d.eventPool.GetEvent(evtag)
	if err != nil {
		return nil, err
	}

	if req.Teams != nil {
		// the requests has a selection of group ids
		for _, reqTeam := range req.Teams {
			lab, ok := ev.GetLabByTeam(reqTeam.Id)
			if !ok {
				return nil, errors.New(fmt.Sprint("team's %s lab not found", reqTeam.Id))
			}

			if err := lab.ResetFrontends(ctx, string(evtag), reqTeam.Id); err != nil {
				return nil, err
			}
		}

		return &pb.ResetTeamStatus{Status: fmt.Sprintf("Frontends are restarted for requested teams.")}, nil
	}

	for _, t := range ev.GetTeams() {
		lab, ok := ev.GetLabByTeam(t.ID())
		if !ok {
			return nil, errors.New(fmt.Sprint("team's %s lab not found", t.ID()))
		}

		if err := lab.ResetFrontends(ctx, string(evtag), t.ID()); err != nil {
			return nil, err
		}
	}

	return &pb.ResetTeamStatus{Status: fmt.Sprintf("Frontends are restarted on event %s", string(evtag))}, nil

}

func (d *daemon) ListFrontends(ctx context.Context, req *pb.Empty) (*pb.ListFrontendsResponse, error) {
	var respList []*pb.ListFrontendsResponse_Frontend

	err := filepath.Walk(d.conf.ConfFiles.OvaDir, func(path string, info os.FileInfo, err error) error {
		if filepath.Ext(path) == ".ova" {
			relativePath, err := filepath.Rel(d.conf.ConfFiles.OvaDir, path)
			if err != nil {
				return err
			}
			parts := strings.Split(relativePath, ".")
			image := filepath.Join(parts[:len(parts)-1]...)

			ic := d.frontends.GetFrontends(image)[0]
			respList = append(respList, &pb.ListFrontendsResponse_Frontend{
				Image:    image,
				Size:     info.Size(),
				MemoryMB: int64(ic.MemoryMB),
				Cpu:      float32(ic.CPU),
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &pb.ListFrontendsResponse{Frontends: respList}, nil
}
