package amigo

import (
	"encoding/json"
	"github.com/aau-network-security/haaukins"
	"github.com/aau-network-security/haaukins/store"
	"time"
)
type Message struct {
	Message string      `json:"msg"`
	Values  interface{} `json:"values"`
}


type TeamRow struct {
	Id          		string       `json:"id"`
	Name        		string       `json:"name"`
	TotalPoints 		uint		 `json:"tpoints"`
	ChalCompletions 	[]*time.Time `json:"completions"`
	ChalPoints 			[]uint 		 `json:"points"`
	IsUser      		bool         `json:"is_user"`
}

type Scoreboard struct {
	Chals 	[]string	`json:"challenges"`
	TeamRow []TeamRow	`json:"teams"`
}

func (fd *FrontendData) initTeams(teamId string) []byte {

	teams := fd.ts.GetTeams()
	rows := make([]TeamRow, len(teams))

	chalsHelper := make([]store.FlagConfig, len(fd.challenges))
	chals := make([]string, len(fd.challenges))
	for j, c := range fd.challenges {
		chalsHelper[j] = c
		chals[j] = c.Name
	}

	for i, t := range teams {
		r := TeamRowFromTeam(t, chalsHelper)
		if t.ID() == teamId {
			r.IsUser = true
		}
		rows[i] = r
	}


	msg := Message{
		Message: "scoreboard",
		Values:  Scoreboard{
			Chals:   chals,
			TeamRow: rows,
		},
	}
	rawMsg, _ := json.Marshal(msg)

	return rawMsg
	//teams := fd.ts.GetTeams()
	//rows := make([]TeamRow, len(teams))
	//for i, t := range teams {
	//	r := TeamRowFromTeam(t)
	//	if t.ID() == teamId {
	//		r.IsUser = true
	//	}
	//	rows[i] = r
	//}
	//
	//msg := Message{
	//	Message: "teams",
	//	Values:  rows,
	//}
	//rawMsg, _ := json.Marshal(msg)
	//
	//return rawMsg
}

func TeamRowFromTeam(t *haaukins.Team, chals []store.FlagConfig) TeamRow {
	completions := make([]*time.Time, len(chals))
	points := make([]uint, len(chals))
	var totalPoints uint = 0
	for i, c := range chals {
		solved := t.IsTeamSolvedChallenge(string(c.Tag))
		completions[i] = solved
		points[i] = c.Points
		if solved != nil {
			totalPoints += c.Points
		}
	}

	return TeamRow{
		Id:          		t.ID(),
		Name:        		t.Name(),
		ChalCompletions: 	completions,
		ChalPoints:		 	points,
		TotalPoints: 		totalPoints,
	}
}

//func TeamRowFromTeam(t *haaukins.Team) TeamRow {
//	chals := t.GetChallenges()
//	completions := make([]*time.Time, len(chals))
//	for i, chal := range chals {
//		completions[i] = chal.CompletedAt
//	}
//
//	return TeamRow{
//		Id:          t.ID(),
//		Name:        t.Name(),
//		Completions: completions,
//	}
//}

type ChallengeF struct {
	ChalInfo  store.FlagConfig		`json:"challenge"`
	IsUserCompleted bool			`json:"isUserCompleted"`
	TeamsCompleted []TeamsCompleted	`json:"teamsCompleted"`
}

type TeamsCompleted struct {
	TeamName	string		`json:"teamName"`
	CompletedAt	*time.Time	`json:"completedAt"`
}

func (fd *FrontendData) initChallenges(teamId string) []byte {
	team, err := fd.ts.GetTeamByID(teamId)
	teams := fd.ts.GetTeams()
	rows := make([]ChallengeF, len(fd.challenges))

	for i, c := range fd.challenges {
		r :=  ChallengeF{
			ChalInfo:        c,
		}

		//check which teams has solve a specif challenge
		for _, t := range teams {
			solved := t.IsTeamSolvedChallenge(string(c.Tag))
			if solved != nil{
				r.TeamsCompleted = append(r.TeamsCompleted, TeamsCompleted{
					TeamName:    t.Name(),
					CompletedAt: solved,
				})
			}
		}

		//check which challenge the user looged in has solved
		if err == nil {
			if team.IsTeamSolvedChallenge(string(c.Tag)) != nil{
				r.IsUserCompleted = true
			}
		}

		rows[i] = r
	}

	msg := Message{
		Message: "challenges",
		Values:  rows,
	}
	chalMsg, _ := json.Marshal(msg)
	return chalMsg
}
