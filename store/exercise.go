package store

import (
	"errors"
	"fmt"

	"github.com/aau-network-security/haaukins/virtual/docker"
)

var (
	EmptyExTags         = errors.New("Exercise cannot have zero tags")
	ImageNotDefinedErr  = errors.New("image cannot be empty")
	MemoryNotDefinedErr = errors.New("memory cannot be empty")
)

type UnknownExerTagErr struct {
	tag Tag
}

func (uee *UnknownExerTagErr) Error() string {
	return fmt.Sprintf("Unknown exercise tag: %s", uee.tag)
}

type ExerTagExistsErr struct {
	tag string
}

func (eee *ExerTagExistsErr) Error() string {
	return fmt.Sprintf("Tag already exists: %s", eee.tag)
}

func (e Exercise) Flags() []ChildrenChalConfig {
	var res []ChildrenChalConfig
	for _, conf := range e.Instance {
		isStatic := e.Static
		for _, f := range conf.Flags {
			f.StaticChallenge = isStatic
			res = append(res, f)
		}
	}
	return res
}

func (e Exercise) ChildTags() []string {
	var childTags []string
	for _, i := range e.Flags() {
		childTags = append(childTags, string(i.Tag))
	}
	return childTags
}

func (e Exercise) Validate() error {
	if e.Tag == "" {
		return &EmptyVarErr{Var: "Tag", Type: "Exercise"}
	}

	return nil
}

type ContainerOptions struct {
	DockerConf docker.ContainerConfig
	Records    []RecordConfig
	Challenges []Challenge
}

func (e Exercise) ContainerOpts() []ContainerOptions {
	var opts []ContainerOptions

	for _, conf := range e.Instance {
		var challenges []Challenge
		envVars := make(map[string]string)

		for _, flag := range conf.Flags {
			value := flag.StaticFlag

			// static flag format in exercises file
			//  should obey flag format HKN{*********}
			if value == "" {
				// flag is not static
				value = NewFlag().String()
				envVars[flag.EnvVar] = value
			}

			challenges = append(challenges, Challenge{
				Name:  flag.Name,
				Tag:   flag.Tag,
				Value: value,
			})

		}

		for _, env := range conf.Envs {
			envVars[env.EnvVar] = env.Value
		}

		// docker config

		spec := docker.ContainerConfig{}

		if !e.Static {
			spec = docker.ContainerConfig{
				Image: conf.Image,
				Resources: &docker.Resources{
					MemoryMB: conf.MemoryMB,
					CPU:      conf.CPU,
				},
				EnvVars: envVars,
			}
		}

		opts = append(opts, ContainerOptions{
			DockerConf: spec,
			Records:    conf.Records,
			Challenges: challenges,
		})
	}

	return opts
}

func (rc RecordConfig) Validate() error {
	if rc.Type == "" {
		return &EmptyVarErr{Var: "Type", Type: "Record config"}
	}

	if rc.Name == "" {
		return &EmptyVarErr{Var: "Name", Type: "Record config"}
	}

	return nil
}

func (rc RecordConfig) Format(ip string) string {
	return fmt.Sprintf("%s %s %s", rc.Name, rc.Type, ip)
}

func (fc ChildrenChalConfig) Validate() error {
	if err := fc.Tag.Validate(); err != nil {
		return err
	}

	if fc.Name == "" {
		return &EmptyVarErr{Var: "Name", Type: "Flag ServiceConfig"}
	}

	if fc.StaticFlag == "" && fc.EnvVar == "" {
		return &EmptyVarErr{Var: "Static or Env", Type: "Flag ServiceConfig"}
	}

	if fc.Points == 0 {
		return &EmptyVarErr{Var: "Points", Type: "Flag ServiceConfig"}
	}

	return nil
}

func (evc EnvVarConfig) Validate() error {
	if evc.EnvVar == "" {
		return &EmptyVarErr{Var: "Env", Type: "Environment Variable"}
	}

	if evc.Value == "" {
		return &EmptyVarErr{Var: "Value", Type: "Environment Variable"}
	}

	return nil
}

func (ic InstanceConfig) Validate() error {
	if ic.Image == "" {
		return &EmptyVarErr{Var: "Image", Type: "Instance ServiceConfig"}
	}
	if ic.MemoryMB < 0 {
		return errors.New("memory cannot be negative")
	}
	if ic.CPU < 0 {
		return errors.New("cpu cannot be negative")
	}

	return nil
}

type VboxConfig struct {
	ExerciseInstanceConfig `yaml:",inline"`
}

func (vc VboxConfig) Validate() error {
	if vc.Image == "" {
		return ImageNotDefinedErr
	}
	if vc.MemoryMB == 0 {
		return MemoryNotDefinedErr
	}
	return nil
}
