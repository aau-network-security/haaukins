package store

type Tag string

type Category struct {
	Tag            Tag    `json:"tag,omitempty"`
	Name           string `json:"name,omitempty"`
	CatDescription string `json:"catDesc,omitempty"`
}

type Profile struct {
	Name       string       `json:"name,omitempty"`
	Secret     bool       `json:"secret,omitempty"`
	Challenges []PChallenge `json:"challenges,omitempty"`
}

type PChallenge struct {
	Tag  string `json:"tag,omitempty"`
	Name string `json:"name,omitempty"`
}

//todo manage the status somehow
type Exercise struct {
	Tag      Tag    `json:"tag,omitempty"`
	Name     string `json:"name,omitempty"`
	Category string `json:"category,omitempty"`
	Secret   bool   `json:"secret,omitempty"`
	// specifies whether challenge will be on docker/vm or none
	// true: none , false: docker/vm
	Static         bool                     `json:"static,omitempty"`
	Instance       []ExerciseInstanceConfig `json:"instance,omitempty"`
	Status         int                      `json:"status,omitempty"`
	OrgDescription string                   `json:"organizerDescription,omitempty"`
}

type ExerciseInstanceConfig struct {
	Image    string               `json:"image,omitempty"`
	MemoryMB uint                 `json:"memory,omitempty"`
	CPU      float64              `json:"cpu,omitempty"`
	Envs     []EnvVarConfig       `json:"envs,omitempty"`
	Flags    []ChildrenChalConfig `json:"children,omitempty"`
	Records  []RecordConfig       `json:"records,omitempty"`
}

type ChildrenChalConfig struct {
	Tag             Tag      `json:"tag,omitempty"`
	Name            string   `json:"name,omitempty"`
	EnvVar          string   `json:"envFlag,omitempty"`
	StaticFlag      string   `json:"static,omitempty"`
	Points          uint     `json:"points,omitempty"`
	Category        string   `json:"category,omitempty"`
	TeamDescription string   `json:"teamDescription,omitempty"`
	PreRequisites   []string `json:"prerequisite,omitempty"`
	Outcomes        []string `json:"outcome,omitempty"`
	StaticChallenge bool     `json:"staticChallenge,omitempty"`
}

type RecordConfig struct {
	Type  string `json:"type,omitempty"`
	Name  string `json:"name,omitempty"`
	RData string `json:"data,omitempty"`
}

type EnvVarConfig struct {
	EnvVar string `json:"name,omitempty"`
	Value  string `json:"value,omitempty"`
}

type InstanceConfig struct {
	Image    string  `yaml:"image"`
	MemoryMB uint    `yaml:"memoryMB"`
	CPU      float64 `yaml:"cpu"`
}
