package daemon

import (
	"context"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
	"net/http"

	"time"

	pb "github.com/aau-network-security/haaukins/daemon/proto"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

var upgrader = websocket.Upgrader{}

func monitorHost(w http.ResponseWriter, r *http.Request) {

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()
	for {

		var cpuErr string
		var cpuPercent float32
		cpus, err := cpu.Percent(time.Second, false)
		if err != nil {
			cpuErr = err.Error()
			log.Error().Err(err).Msgf("error getting cpu percent %v", cpuErr)
		}
		if len(cpus) == 1 {
			cpuPercent = float32(cpus[0])
		}
		log.Debug().Msgf("CPU: %f", cpuPercent)

		var memErr string
		v, err := mem.VirtualMemory()
		log.Debug().Msgf("Memory: %f", v.UsedPercent)
		if err != nil {
			memErr = err.Error()
			log.Error().Msgf("Monitor memory error: %s", memErr)
		}

		message := "{\"cpuPercent\":\"" + fmt.Sprintf("%f", cpuPercent) + "\",\"memPercent\":\"" + fmt.Sprintf("%f", v.UsedPercent) + "\",\"memError\":\"" + memErr + "\",\"cpuError\":\"" + cpuErr + "\"}"

		log.Debug().Msgf("Sending message: %s", string(message))

		err = c.WriteMessage(websocket.TextMessage, []byte(message))
		if err != nil {
			log.Error().Msgf("error writing to websocket: %s", err)
			break
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
