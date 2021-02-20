package store

type Tag string

type Category struct {
	Tag  Tag    `json:"tag,omitempty"`
	Name string `json:"name,omitempty"`
}

//todo manage the status somehow
type Exercise struct {
	Tag      Tag                      `json:"tag,omitempty"`
	Name     string                   `json:"name,omitempty"`
	Category string                   `json:"category,omitempty"`
	Instance []ExerciseInstanceConfig `json:"instance,omitempty"`
	Status   int                      `json:"status,omitempty"`
}

type ExerciseInstanceConfig struct {
	Image    string         `json:"image,omitempty"`
	MemoryMB uint           `json:"memory,omitempty"`
	CPU      float64        `json:"cpu,omitempty"`
	Envs     []EnvVarConfig `json:"envs,omitempty"`
	Flags    []FlagConfig   `json:"children,omitempty"`
	Records  []RecordConfig `json:"records,omitempty"`
}

type FlagConfig struct {
	Tag             Tag      `json:"tag,omitempty"`
	Name            string   `json:"name,omitempty"`
	EnvVar          string   `json:"envFlag,omitempty"`
	StaticFlag      string   `json:"static_flag,omitempty"`
	Points          uint     `json:"points,omitempty"`
	Category        string   `json:"category,omitempty"`
	TeamDescription string   `json:"teamDescription,omitempty"`
	OrgDescription  string   `json:"organizerDescription,omitempty"`
	PreRequisites   []string `json:"prerequisite,omitempty"`
	Outcomes        []string `json:"outcome,omitempty"`
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
