package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

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
		if !usr.SuperUser && estruct.Secret {
			continue
		}
		if d.conf.ProductionMode && e.Status == 1 {
			// do not include exercises which are in test mode
			// if production mode active
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

			t, err := ev.GetTeamById(reqTeam.Id)
			teamDisabledMap := t.GetDisabledChalMap()
			if err != nil {
				log.Printf("GetTeamById error no team found %v", err)
				continue
			}

			if teamDisabledMap != nil {
				_, ok = teamDisabledMap[req.ExerciseTag]
				if ok {
					if t.ManageDisabledChals(req.ExerciseTag) {
						log.Printf("Disabled exercises updated [ %s ] removed from disabled exercises via gRPC for team [ %s ] ", req.ExerciseTag, t.ID())
					}
				}
			}
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

func (d *daemon) GetExercisesByTags(ctx context.Context, req *pb.GetExsByTagsReq) (*pb.GetExsByTagsResp, error) {
	var exInfo []*pb.GetExsByTagsResp_ExInfo
	exTags := req.Tags
	resp, err := d.exClient.GetExerciseByTags(ctx, &eproto.GetExerciseByTagsRequest{Tag: exTags})
	if err != nil {
		return &pb.GetExsByTagsResp{}, err
	}
	for _, e := range resp.Exercises {
		exInfo = append(exInfo, &pb.GetExsByTagsResp_ExInfo{Tag: e.Tag, Name: e.Name})
	}
	return &pb.GetExsByTagsResp{Exercises: exInfo}, nil
}

func protobufToJson(message proto.Message) (string, error) {
	marshaler := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: false,
		Indent:       "  ",
	}

	return marshaler.MarshalToString(message)
}

func (d *daemon) AddChallenge(ctx context.Context, req *pb.AddChallengeRequest) (*pb.AddChallengeResponse, error) {
	eventTag := req.EventTag
	challengeTags := req.ChallengeTag
	var exers []store.Exercise
	var waitGroup sync.WaitGroup
	var once sync.Once

	ev, err := d.eventPool.GetEvent(store.Tag(eventTag))
	if err != nil {
		return &pb.AddChallengeResponse{Message: err.Error()}, err
	}

	exer, err := d.exClient.GetExerciseByTags(ctx, &eproto.GetExerciseByTagsRequest{Tag: challengeTags})
	if err != nil {
		return nil, fmt.Errorf("[exercises-service] error %v", err)
	}

	for _, e := range exer.Exercises {
		exercise, err := protobufToJson(e)
		if err != nil {
			return nil, err
		}
		estruct := store.Exercise{}
		json.Unmarshal([]byte(exercise), &estruct)
		exers = append(exers, estruct)
	}
	var addChalError error
	go func() {
		lb := <-ev.GetHub().Queue()
		if err := lb.AddChallenge(ctx, exers...); err != nil {
			addChalError = err
		}
	}()
	frontendData := ev.GetFrontendData()
	for tid, l := range ev.GetAssignedLabs() {
		if err := l.AddChallenge(ctx, exers...); err != nil {
			addChalError = err
		}
		t, err := ev.GetTeamById(tid)
		if err != nil {
			addChalError = err
		}

		chals := l.Environment().Challenges()
		for _, chal := range chals {
			tag, _ := store.NewTag(string(chal.Tag))
			_, _ = t.AddChallenge(store.Challenge{
				Tag:   tag,
				Name:  chal.Name,
				Value: chal.Value,
			})
			log.Info().Str("chal-tag", string(tag)).
				Str("chal-val", chal.Value).
				Msgf("Flag is created for team %s [add-challenge function] ", t.Name())
		}

	}
	updateChallenges := func() {
		for _, fl := range exers {
			waitGroup.Add(1)
			go func() {
				defer waitGroup.Done()
				frontendData.UpdateChallenges(fl.Flags()...)

			}()
			waitGroup.Wait()
		}
	}
	once.Do(updateChallenges)

	if addChalError != nil {
		return &pb.AddChallengeResponse{Message: fmt.Sprintf("Error: %v", addChalError)}, addChalError
	}
	return &pb.AddChallengeResponse{Message: fmt.Sprintf("challenges %v are added to event %s ", challengeTags, eventTag)}, nil
}
