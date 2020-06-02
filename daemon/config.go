// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package daemon

import (
	dockerclient "github.com/fsouza/go-dockerclient"
)

type Config struct {
	Host struct {
		Http string `yaml:"http,omitempty"`
		Grpc string `yaml:"grpc,omitempty"`
	} `yaml:"host,omitempty"`
	Port struct {
		Secure   uint `yaml:"secure,omitempty"`
		InSecure uint `yaml:"insecure,omitempty"`
	}
	Certs              CertificateConfig                `yaml:"tls,omitempty"`
	Database           DatabaseConfig                   `yaml:"db-config,omitempty"`
	ConfFiles          Files                            `yaml:"files,omitempty"`
	SigningKey         string                           `yaml:"sign-key,omitempty"`
	DockerRepositories []dockerclient.AuthConfiguration `yaml:"docker-repositories,omitempty"`
}

type CertificateConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Directory string `yaml:"directory"`
	CertFile  string `yaml:"certfile"`
	CertKey   string `yaml:"certkey"`
	CAFile    string `yaml:"cafile"`
}

type DatabaseConfig struct {
	Grpc       string            `yaml:"grpc,omitempty"`
	AuthKey    string            `yaml:"db-auth-key,omitempty"`
	SignKey    string            `yaml:"db-sign-key,omitempty"`
	CertConfig CertificateConfig `yaml:"tls,omitempty"`
}

type Files struct {
	OvaDir        string `yaml:"ova-directory,omitempty"`
	LogDir        string `yaml:"log-directory,omitempty"`
	EventsDir     string `yaml:"events-directory,omitempty"`
	UsersFile     string `yaml:"users-file,omitempty"`
	ExercisesFile string `yaml:"exercises-file,omitempty"`
	FrontendsFile string `yaml:"frontends-file,omitempty"`
}
