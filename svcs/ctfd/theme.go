// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package ctfd

type Theme struct {
	ExtraFields *ExtraFields
	Index       string
	CSS         string
}

var (
	survey, _ = NewExtraFields([]InputRow{
		{
			Class: "form-group",
			Inputs: []Input{
				NewSelector("Team Size", "team-size", []string{"1", "2", "3", "4", "5", "6", "7", "8", "9+"}),
				NewSelector("Technology Interest", "tech-interest", []string{"We enjoy technology", "Not interested in technology"}),
			},
		},
		{
			Class: "form-group",
			Inputs: []Input{
				NewSelector("Hacking Experience (in total)", "hack-exp", []string{"None", "1-2 years", "3-4 years", "5-8 years", "9+ years"}),
			},
		},
		{
			Class: "form-check",
			Inputs: []Input{
				NewCheckbox("consent", `I hereby declare that I understand and agree that 
(1) my activity (i.e. key presses and mouse clicks) on the platform is being monitored,
(2) the data is anonymised and stored securely and 
(3) the raw data will NOT be shared with other parties and may be shared within the scientific community in a processed form.`, true),
			},
		},
	})

	Themes = map[string]Theme{
		"aau": Theme{
			CSS: aauCss,
		},
		"aau-survey": Theme{
			ExtraFields: survey,
			CSS:         aauCss,
		},
	}
)
