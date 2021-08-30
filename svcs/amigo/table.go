package amigo

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/aau-network-security/haaukins/store"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/parser"
	"github.com/microcosm-cc/bluemonday"
)

type Message struct {
	Message       string      `json:"msg"`
	Values        interface{} `json:"values"`
	IsLabAssigned bool        `json:"isLabAssigned"`
}

// Challenge name and the points relative that challenge
type Challenge struct {
	Name   string `json:"name"`
	Tag    string `json:"tag"`
	Points uint   `json:"points"`
}

// Contains a list of Challenges relative the CategoryName
type Category struct {
	CategoryName string      `json:"category"`
	Challenges   []Challenge `json:"chals"`
}

type TeamRow struct {
	Id              string       `json:"id"`
	Name            string       `json:"name"`
	TotalPoints     uint         `json:"tpoints"`
	ChalCompletions []*time.Time `json:"completions"`
	ChalPoints      []uint       `json:"points"`
	IsUser          bool         `json:"is_user"`
}

type Scoreboard struct {
	Category []Category `json:"challenges"`
	TeamRow  []TeamRow  `json:"teams"`
}

// Retrieve categories directly from given challenges
//  mapping is required to prevent duplicate values in
// returning list
func (fd *FrontendData) getChallengeCategories() []string {
	keys := make(map[string]bool)
	var challengeCats []string
	for _, challenge := range fd.challenges {
		if _, value := keys[challenge.Category]; !value {
			keys[challenge.Category] = true
			challengeCats = append(challengeCats, challenge.Category)
		}
	}
	return challengeCats
}

func (fd *FrontendData) initTeams(teamId string) []byte {

	teams := fd.ts.GetTeams()
	rows := make([]TeamRow, len(teams))
	var challenges []Category

	// this part contains a lot of loops
	// in my opinion structs are not well defined

	for _, c := range fd.getChallengeCategories() {
		challenges = append(challenges, Category{
			CategoryName: c,
			Challenges:   []Challenge{},
		})
	}

	for _, c := range fd.challenges {
		for i, rc := range challenges {
			if rc.CategoryName == c.Category {

				challenges[i].Challenges = append(challenges[i].Challenges, Challenge{
					Name:   c.Name,
					Tag:    string(c.Tag),
					Points: c.Points,
				})
			}
		}
	}

	for _, c := range challenges {
		if len(c.Challenges) > 0 {
			sort.SliceStable(c.Challenges, func(i, j int) bool {
				return c.Challenges[i].Points < c.Challenges[j].Points
			})
		}
	}

	for i, t := range teams {
		r := TeamInfo(t, challenges)
		if t.ID() == teamId {
			r.IsUser = true
		}
		rows[i] = r
	}

	msg := Message{
		Message: "scoreboard",
		Values: Scoreboard{
			Category: challenges,
			TeamRow:  rows,
		},
	}
	rawMsg, _ := json.Marshal(msg)

	return rawMsg
}

func TeamInfo(t *store.Team, chalCategories []Category) TeamRow {
	var completions []*time.Time
	var points []uint
	var totalPoints uint = 0
	for _, cc := range chalCategories {
		for _, c := range cc.Challenges {
			solved := t.IsTeamSolvedChallenge(c.Tag)
			completions = append(completions, solved)
			points = append(points, c.Points)
			if solved != nil {
				totalPoints += c.Points
			}
		}
	}

	return TeamRow{
		Id:              t.ID(),
		Name:            t.Name(),
		ChalCompletions: completions,
		ChalPoints:      points,
		TotalPoints:     totalPoints,
	}
}

// Challenge for Challenges Page. It contains the challenge information, which team has solved that challenge and if
// the current user has solve that challenge
type ChallengeCP struct {
	ChalInfo        store.ChildrenChalConfig `json:"challenge"`
	IsUserCompleted bool                     `json:"isUserCompleted"`
	TeamsCompleted  []TeamsCompleted         `json:"teamsCompleted"`
	IsDisabledChal  bool                     `json:"isChalDisabled"`
}

type TeamsCompleted struct {
	TeamName    string     `json:"teamName"`
	CompletedAt *time.Time `json:"completedAt"`
}

func (fd *FrontendData) initChallenges(teamId string) []byte {
	rows := make([]ChallengeCP, len(fd.challenges))
	team, err := fd.ts.GetTeamByID(teamId)
	if err != nil {
		msg := Message{
			Message:       "challenges",
			Values:        rows,
			IsLabAssigned: false,
		}
		chalMsg, _ := json.Marshal(msg)
		return chalMsg
	}
	teams := fd.ts.GetTeams()
	isTeamAssigned := team.IsLabAssigned()

	for i, c := range fd.challenges {
		r := ChallengeCP{
			ChalInfo: store.ChildrenChalConfig{
				Tag:             c.Tag,
				Name:            c.Name,
				Points:          c.Points,
				Category:        c.Category,
				TeamDescription: c.TeamDescription,
				PreRequisites:   c.PreRequisites,
				Outcomes:        c.Outcomes,
				StaticChallenge: c.StaticChallenge,
			},
		}

		//Render markdown to HTML
		extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.HardLineBreak
		parser := parser.NewWithExtensions(extensions)

		md := []byte(r.ChalInfo.TeamDescription)
		unsafeHtml := markdown.ToHTML(md, parser, nil)

		//Sanitizing unsafe HTML with bluemonday
		html := bluemonday.UGCPolicy().SanitizeBytes(unsafeHtml)
		r.ChalInfo.TeamDescription = string(html)

		//check which teams has solve a specif challenge
		for _, t := range teams {
			solved := t.IsTeamSolvedChallenge(string(c.Tag))
			if solved != nil {
				r.TeamsCompleted = append(r.TeamsCompleted, TeamsCompleted{
					TeamName:    t.Name(),
					CompletedAt: solved,
				})
			}
			// check disabled challenges and its children challenges here
			for _, d := range t.GetDisabledChals() {
				if d == string(c.Tag) && solved == nil {
					r.IsDisabledChal = true
				}
			}
		}

		//check which challenge the user looged in has solved
		if err == nil {
			if team.IsTeamSolvedChallenge(string(c.Tag)) != nil {
				r.IsUserCompleted = true
				r.IsDisabledChal = false
			}
		}

		rows[i] = r
	}

	msg := Message{
		Message:       "challenges",
		Values:        rows,
		IsLabAssigned: isTeamAssigned,
	}
	chalMsg, _ := json.Marshal(msg)
	return chalMsg
}
