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

// Challenge name and the points relative that challenge
type Challenge struct {
	Name	string		`json:"name"`
	Points	uint		`json:"points"`
}

// Contains a list of Challenges relative the CategoryName
type Category struct {
	CategoryName		string			`json:"category"`
	Challenges			[]Challenge		`json:"chals"`
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
	Category 	[]Category	`json:"challenges"`
	TeamRow 	[]TeamRow	`json:"teams"`
}

func (fd *FrontendData) initTeams(teamId string) []byte {

	teams := fd.ts.GetTeams()
	rows := make([]TeamRow, len(teams))
	var challenges []Category
	var realChallenges []Category		//used to filter out the empty categories
	
	for _, c := range ChallengeCategories {
		challenges = append(challenges, Category{
			CategoryName: c,
			Challenges: []Challenge{},
		})
	}

	for _, c := range fd.challenges {
		for i, rc := range challenges{
			if rc.CategoryName == c.Category{

				challenges[i].Challenges = append(challenges[i].Challenges, Challenge{
					Name:   c.Name,
					Points: c.Points,
				})
			}
		}
	}

	for _, c := range challenges{
		if len(c.Challenges) > 0 {
			sort.SliceStable(c.Challenges, func(i, j int) bool {
				return c.Challenges[i].Points < c.Challenges[j].Points
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
			Category:   realChallenges,
			TeamRow: rows,
		},
	}
	rawMsg, _ := json.Marshal(msg)

	return rawMsg
}

func TeamInfo(t *store.Team, chalCategories []Category) TeamRow {
	var completions	[]*time.Time
	var points 		[]uint
	var totalPoints uint = 0
	for _, cc := range chalCategories {
		for _, c := range cc.Challenges{
			solved := t.IsTeamSolvedChallenge(c.Name)
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

// Challenge for Challenges Page. It contains the challenge information, which team has solved that challenge and if
// the current user has solve that challenge
type ChallengeCP struct {
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
	rows := make([]ChallengeCP, len(fd.challenges))

	for i, c := range fd.challenges {
		r :=  ChallengeCP{
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
