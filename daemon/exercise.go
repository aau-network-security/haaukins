package daemon

import (
	"context"
	pb "github.com/aau-network-security/haaukins/daemon/proto"
	"github.com/aau-network-security/haaukins/store"
	"github.com/rs/zerolog/log"
)



func (d *daemon) ListExercises(ctx context.Context, req *pb.Empty) (*pb.ListExercisesResponse, error) {
	var exercises []*pb.ListExercisesResponse_Exercise

	for _, e := range d.exercises.ListExercises() {
		var tags []string
		for _, t := range e.Tags {
			tags = append(tags, string(t))
		}

		var exercisesInfo []*pb.ListExercisesResponse_Exercise_ExerciseInfo
		for _, e := range d.exercises.GetExercisesInfo(e.Tags[0]){

			exercisesInfo = append(exercisesInfo, &pb.ListExercisesResponse_Exercise_ExerciseInfo{
				Tag:                  string(e.Tag),
				Name:                 e.Name,
				Points:               int32(e.Points),
				Category:             e.Category,
				Description:          e.Description,
			})
		}

		exercises = append(exercises, &pb.ListExercisesResponse_Exercise{
			Name:             	  e.Name,
			Tags:             	  tags,
			DockerImageCount: 	  int32(len(e.DockerConfs)),
			VboxImageCount:   	  int32(len(e.VboxConfs)),
			Exerciseinfo:         exercisesInfo,
		})
	}

	return &pb.ListExercisesResponse{Exercises: exercises}, nil
}


func (d *daemon) UpdateExercisesFile(ctx context.Context, req *pb.Empty) (*pb.UpdateExercisesFileResponse, error) {
	exercises, err := d.exercises.UpdateExercisesFile(d.conf.ConfFiles.ExercisesFile)
	if err != nil {
		return nil, err
	}
	// update event host exercises store
	if err := d.ehost.UpdateEventHostExercisesFile(exercises); err != nil {
		return nil, err
	}
	// update daemons' exercises store
	d.exercises = exercises
	return &pb.UpdateExercisesFileResponse{
		Msg: "Exercises file updated ",
	}, nil

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