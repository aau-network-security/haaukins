package daemon

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

	"time"

	pb "github.com/aau-network-security/haaukins/daemon/proto"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

var upgrader = websocket.Upgrader{}

func monitorHost(w http.ResponseWriter, r *http.Request) {
	t := time.NewTicker(2 * time.Second)
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()
	for range t.C {

		var cpuErr string
		var cpuPercent float32
		cpus, err := cpu.Percent(time.Second, false)
		if err != nil {
			cpuErr = err.Error()
			log.Error().Err(err).Msgf("error getting cpu percent %v", cpuErr)
			break
		}
		if len(cpus) == 1 {
			cpuPercent = float32(cpus[0])
		}

		var memErr string
		v, err := mem.VirtualMemory()

		if err != nil {
			memErr = err.Error()
			log.Error().Msgf("Monitor memory error: %s", memErr)
			break

		}

		message := "{\"cpuPercent\":\"" + fmt.Sprintf("%f", cpuPercent) + "\",\"memPercent\":\"" + fmt.Sprintf("%f", v.UsedPercent) + "\",\"memError\":\"" + memErr + "\",\"cpuError\":\"" + cpuErr + "\"}"

		err = c.WriteMessage(websocket.TextMessage, []byte(message))
		if err != nil {
			log.Error().Msgf("error writing to websocket: %s", err)
			break
		}
	}
}

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
