package ctfd

type Theme struct {
	ExtraFields *ExtraFields
	Index       string
	CSS         string
}

var (
	Themes = map[string]Theme{
		"aau": Theme{
			CSS: aauCss,
		},
		"aau-survey": Theme{
			ExtraFields: NewExtraFields([][]*Selector{
				{
					NewSelector("Team Size", "team-size", []string{"1", "2", "3", "4", "5", "6", "7", "8", "9+"}),
					NewSelector("Technology Interest", "tech-interest", []string{"We enjoy technology", "Not interested in technology"}),
				},
				{
					NewSelector("Hacking Experience (in total)", "hack-exp", []string{"None", "1-2 years", "3-4 years", "5-8 years", "9+ years"}),
				},
			}),
			CSS: aauCss,
		},
	}
)
