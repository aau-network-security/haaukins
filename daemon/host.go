package daemon

import (
	"context"
	"fmt"

	"time"

	pb "github.com/aau-network-security/haaukins/daemon/proto"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

func (d *daemon) MonitorHost(req *pb.Empty, stream pb.Daemon_MonitorHostServer) error {
	for {
		var cpuErr string
		var cpuPercent float32
		cpus, err := cpu.Percent(time.Second, false)
		if err != nil {
			cpuErr = err.Error()
		}
		if len(cpus) == 1 {
			cpuPercent = float32(cpus[0])
		}

		var memErr string
		v, err := mem.VirtualMemory()
		if err != nil {
			memErr = err.Error()
		}

		//todo we should send io at some point
		// io, _ := net.IOCounters(true)

		if err := stream.Send(&pb.MonitorHostResponse{
			CPUPercent:      cpuPercent,
			CPUReadError:    cpuErr,
			MemoryPercent:   float32(v.UsedPercent),
			MemoryReadError: memErr,
		}); err != nil {
			return err
		}
	}
}
func (d *daemon) GetAPICreds(ctx context.Context, req *pb.Empty) (*pb.CredsResponse, error) {
	usr, err := getUserFromIncomingContext(ctx)
	if err != nil {
		return &pb.CredsResponse{}, err
	}
	if !usr.SuperUser {
		return &pb.CredsResponse{}, fmt.Errorf("No priviledge to see auth keys for API")
	}
	return &pb.CredsResponse{
		Username: d.conf.APICreds.Username,
		Password: d.conf.APICreds.Password,
	}, nil
}
