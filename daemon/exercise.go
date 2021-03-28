package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	pb "github.com/aau-network-security/haaukins/daemon/proto"
	eproto "github.com/aau-network-security/haaukins/exercise/ex-proto"
	"github.com/aau-network-security/haaukins/store"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/rs/zerolog/log"
)

func (d *daemon) ListExercises(ctx context.Context, req *pb.Empty) (*pb.ListExercisesResponse, error) {
	var vboxCount int32
	var exercises []*pb.ListExercisesResponse_Exercise
	usr, err := getUserFromIncomingContext(ctx)
	if err != nil {
		return &pb.ListExercisesResponse{}, NoUserInformation
	}

	exes, err := d.exClient.GetExercises(ctx, &eproto.Empty{})
	log.Debug().Msgf("Request all exercises from the service")
	if err != nil {
		return &pb.ListExercisesResponse{}, fmt.Errorf("[exercise-service]: ERR getting exercises %v", err)
	}

	var exers []store.Exercise

	for _, e := range exes.Exercises {
		exercise, err := protobufToJson(e)
		if err != nil {
			return nil, err
		}
		estruct := store.Exercise{}
		json.Unmarshal([]byte(exercise), &estruct)
		log.Debug().Str("User", usr.Name).Msgf("listed exercises.")
		if !usr.SuperUser && estruct.Secret {
			continue
		}
		if d.conf.ProductionMode && e.Status == 1 {
			// do not include exercises which are in test mode
			// if production mode active
			log.Debug().Bool("Production Mode", d.conf.ProductionMode).Msgf("Message: %v", e)
			continue
		}
		exers = append(exers, estruct)
	}

	for _, e := range exers {

		var tags []string
		tags = append(tags, string(e.Tag))

		var exercisesInfo []*pb.ListExercisesResponse_Exercise_ExerciseInfo

		for _, i := range e.Instance {
			if !strings.Contains(i.Image, d.conf.DockerRepositories[0].ServerAddress) {
				vboxCount++
			}
			for _, c := range i.Flags {
				exercisesInfo = append(exercisesInfo, &pb.ListExercisesResponse_Exercise_ExerciseInfo{
					Tag:         string(c.Tag),
					Name:        c.Name,
					Points:      int32(c.Points),
					Category:    c.Category,
					Description: c.TeamDescription,
				})
			}

		}

		exercises = append(exercises, &pb.ListExercisesResponse_Exercise{
			Name:             e.Name,
			Tags:             tags,
			Secret:           e.Secret,
			DockerImageCount: int32(len(e.Instance)),
			VboxImageCount:   vboxCount,
			Exerciseinfo:     exercisesInfo,
		})
	}

	return &pb.ListExercisesResponse{Exercises: exercises}, nil
}

func (d *daemon) ResetExercise(req *pb.ResetExerciseRequest, stream pb.Daemon_ResetExerciseServer) error {
	log.Ctx(stream.Context()).Info().
		Str("evtag", req.EventTag).
		Str("extag", req.ExerciseTag).
		Msg("reset exercise")

	evtag, err := store.NewTag(req.EventTag)
	if err != nil {
		return err
	}

	ev, err := d.eventPool.GetEvent(evtag)
	if err != nil {
		return err
	}

	if req.Teams != nil {
		for _, reqTeam := range req.Teams {
			lab, ok := ev.GetLabByTeam(reqTeam.Id)
			if !ok {
				stream.Send(&pb.ResetTeamStatus{TeamId: reqTeam.Id, Status: "?"})
				continue
			}

			if err := lab.Environment().ResetByTag(stream.Context(), req.ExerciseTag); err != nil {
				return err
			}
			stream.Send(&pb.ResetTeamStatus{TeamId: reqTeam.Id, Status: "ok"})
		}

		return nil
	}

	for _, t := range ev.GetTeams() {
		lab, ok := ev.GetLabByTeam(t.ID())
		if !ok {
			stream.Send(&pb.ResetTeamStatus{TeamId: t.ID(), Status: "?"})
			continue
		}

		if err := lab.Environment().ResetByTag(stream.Context(), req.ExerciseTag); err != nil {
			return err
		}
		stream.Send(&pb.ResetTeamStatus{TeamId: t.ID(), Status: "ok"})
	}

	return nil
}

func protobufToJson(message proto.Message) (string, error) {
	marshaler := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: false,
		Indent:       "  ",
	}

	return marshaler.MarshalToString(message)
}
