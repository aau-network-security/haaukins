package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	pb "github.com/aau-network-security/haaukins/daemon/proto"
	eproto "github.com/aau-network-security/haaukins/exercise/ex-proto"
	"github.com/aau-network-security/haaukins/store"
	storeProto "github.com/aau-network-security/haaukins/store/proto"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/parser"
	"github.com/microcosm-cc/bluemonday"
	"github.com/rs/zerolog/log"
)

func (d *daemon) ListCategories(ctx context.Context, req *pb.Empty) (*pb.ListCategoriesResponse, error) {
	var categories []*pb.ListCategoriesResponse_Category
	_, err := getUserFromIncomingContext(ctx)
	if err != nil {
		return &pb.ListCategoriesResponse{}, NoUserInformation
	}

	categs, err := d.exClient.GetCategories(ctx, &eproto.Empty{})
	if err != nil {
		return &pb.ListCategoriesResponse{}, fmt.Errorf("[exercise-service]: ERR getting categories %v", err)
	}
	var cats []store.Category

	for _, c := range categs.Categories {
		category, err := protobufToJson(c)
		if err != nil {
			return nil, err
		}
		cstruct := store.Category{}
		json.Unmarshal([]byte(category), &cstruct)
		cats = append(cats, cstruct)
	}

	for _, c := range cats {
		// Render markdown from orgdescription to html
		extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.HardLineBreak
		parser := parser.NewWithExtensions(extensions)

		md := []byte(c.CatDescription)
		unsafeHtml := markdown.ToHTML(md, parser, nil)

		//Sanitizing unsafe HTML with bluemonday
		html := bluemonday.UGCPolicy().SanitizeBytes(unsafeHtml)
		c.CatDescription = string(html)

		categories = append(categories, &pb.ListCategoriesResponse_Category{
			Tag:            string(c.Tag),
			Name:           c.Name,
			CatDescription: c.CatDescription,
		})
	}

	return &pb.ListCategoriesResponse{Categories: categories}, nil
}
func (d *daemon) ListExercises(ctx context.Context, req *pb.Empty) (*pb.ListExercisesResponse, error) {
	var vboxCount int32
	var exercises []*pb.ListExercisesResponse_Exercise
	usr, err := getUserFromIncomingContext(ctx)
	if err != nil {
		return &pb.ListExercisesResponse{}, NoUserInformation
	}

	exes, err := d.exClient.GetExercises(ctx, &eproto.Empty{})
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

		// Render markdown from orgdescription to html
		extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.HardLineBreak
		parser := parser.NewWithExtensions(extensions)

		md := []byte(e.OrgDescription)
		unsafeHtml := markdown.ToHTML(md, parser, nil)

		//Sanitizing unsafe HTML with bluemonday
		html := bluemonday.UGCPolicy().SanitizeBytes(unsafeHtml)
		e.OrgDescription = string(html)

		exercises = append(exercises, &pb.ListExercisesResponse_Exercise{
			Name:             e.Name,
			Tags:             tags,
			Secret:           e.Secret,
			DockerImageCount: int32(len(e.Instance)),
			VboxImageCount:   vboxCount,
			Exerciseinfo:     exercisesInfo,
			Orgdescription:   e.OrgDescription,
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

// todo: contains too much functions and for loop, requires some optimization
func (d *daemon) AddChallenge(req *pb.AddChallengeRequest, srv pb.Daemon_AddChallengeServer) error {
	eventTag := req.EventTag
	challengeTags := req.ChallengeTag
	ctx := context.TODO()
	var exers []store.Exercise
	var waitGroup sync.WaitGroup
	var once sync.Once
	childrenChalTags := make(map[string][]string)

	ev, err := d.eventPool.GetEvent(store.Tag(eventTag))
	if err != nil {
		return err
	}
	allChals := ev.GetConfig().AllChallenges
	for _, i := range challengeTags {
		_, ok := allChals[i]
		if ok {
			return errors.New(fmt.Sprintf("Requested challenge(s) %v is/are already exists on event", challengeTags))
		}
	}

	exer, err := d.exClient.GetExerciseByTags(ctx, &eproto.GetExerciseByTagsRequest{Tag: challengeTags})
	if err != nil {
		return fmt.Errorf("[exercises-service] error %v", err)
	}

	for _, e := range exer.Exercises {
		exercise, err := protobufToJson(e)
		if err != nil {
			return err
		}
		estruct := store.Exercise{}
		json.Unmarshal([]byte(exercise), &estruct)
		exers = append(exers, estruct)
	}
	// getting all children childs in given parent challenge
	for _, i := range exers {
		for _, parentTag := range challengeTags {
			if string(i.Tag) == parentTag {
				childrenChalTags[parentTag] = i.ChildTags()
			}
		}
	}

	ev.PauseSignup(true)
	var addChalError error

	for _, lb := range ev.GetHub().Labs() {
		waitGroup.Add(1)
		go func() {
			if err := lb.AddChallenge(ctx, exers...); err != nil {
				addChalError = err
			}
			waitGroup.Done()
		}()
		waitGroup.Wait()
	}
	ev.PauseSignup(false)

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
		t.UpdateAllChallenges(childrenChalTags)
	}

	srv.Send(&pb.AddChallengeResponse{Message: "Assigned labs are processing to add challenge(s) ..."})
	delimeter := ","
	resp, err := d.dbClient.UpdateExercises(ctx, &storeProto.UpdateExerciseRequest{
		EventTag:   req.EventTag,
		Challenges: delimeter + strings.Join(req.ChallengeTag, delimeter),
	})
	if err != nil {
		return err
	}
	log.Printf(resp.Message)

	ev.GetHub().UpdateExercises(exers)

	updateChallenges := func() {
		for _, fl := range exers {
			waitGroup.Add(1)
			go func() {
				defer waitGroup.Done()
				frontendData.UpdateChallenges(fl.Flags())

			}()
			waitGroup.Wait()
		}
	}
	once.Do(updateChallenges)

	if addChalError != nil {
		return addChalError
	}

	srv.Send(&pb.AddChallengeResponse{Message: fmt.Sprintf("Challenge(s) %v is/(are) processing for event %s ", challengeTags, eventTag)})
	return nil
}
