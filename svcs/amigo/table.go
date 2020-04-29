package amigo

import (
	"encoding/json"
	"github.com/aau-network-security/haaukins/store"
	"sort"
	"time"
)

var (
	ChallengeCategories = [5]string{"Web exploitation", "Forensics", "Cryptography", "Binary", "Reverse Engineering"}
)

type Message struct {
	Message string      `json:"msg"`
	Values  interface{} `json:"values"`
}

type Chal struct {
	Chal	string		`json:"name"`
	Points	uint		`json:"points"`
}

type ChalCategory struct {
	Category		string		`json:"category"`
	Chals			[]Chal		`json:"chals"`
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
	Chals 	[]ChalCategory	`json:"challenges"`
	TeamRow []TeamRow	`json:"teams"`
}

func (fd *FrontendData) initTeams(teamId string) []byte {

	teams := fd.ts.GetTeams()
	rows := make([]TeamRow, len(teams))
	var challenges []ChalCategory
	var realChallenges []ChalCategory
	
	for _, c := range ChallengeCategories {
		challenges = append(challenges, ChalCategory{
			Category: c,
			Chals: []Chal{},
		})
	}

	for _, c := range fd.challenges {
		for i, rc := range challenges{
			if rc.Category == c.Category{

				challenges[i].Chals = append(challenges[i].Chals, Chal{
					Chal:   c.Name,
					Points: c.Points,
				})
			}
		}
	}

	for _, c := range challenges{
		if len(c.Chals) > 0 {
			sort.SliceStable(c.Chals, func(i, j int) bool {
				return c.Chals[i].Points < c.Chals[j].Points
			})
			realChallenges = append(realChallenges, c)
		}
	}

	for i, t := range teams {
		r := TeamInfo(t, realChallenges)
		if t.ID() == teamId {
			r.IsUser = true
		}
		rows[i] = r
	}


	msg := Message{
		Message: "scoreboard",
		Values:  Scoreboard{
			Chals:   realChallenges,
			TeamRow: rows,
		},
	}
	rawMsg, _ := json.Marshal(msg)

	return rawMsg
}

func TeamInfo(t *store.Team, chalCategories []ChalCategory) TeamRow {
	var completions	[]*time.Time
	var points 		[]uint
	var totalPoints uint = 0
	for _, cc := range chalCategories {
		for _, c := range cc.Chals{
			solved := t.IsTeamSolvedChallenge(c.Chal)
			completions = append(completions, solved)
			points = append(points, c.Points)
			if solved != nil {
				totalPoints += c.Points
			}
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

type ChallengeF struct {
	ChalInfo        store.FlagConfig `json:"challenge"`
	IsUserCompleted bool              `json:"isUserCompleted"`
	TeamsCompleted  []TeamsCompleted  `json:"teamsCompleted"`
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
