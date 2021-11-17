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
	Database           ServiceConfig                    `yaml:"db-config,omitempty"`
	ExerciseService    ServiceConfig                    `yaml:"exercise-service,omitempty"`
	VPNConn            VPNConnConf                      `yaml:"vpn-service,omitempty"`
	ConfFiles          Files                            `yaml:"files,omitempty"`
	ProductionMode     bool                             `yaml:"prodmode,omitempty"`
	SigningKey         string                           `yaml:"sign-key,omitempty"`
	Rechaptcha         string                           `yaml:"recaptcha-key,omitempty"`
	APICreds           APICreds                         `yaml:"api-creds,omitempty"`
	DockerRepositories []dockerclient.AuthConfiguration `yaml:"docker-repositories,omitempty"`
	FileTransferRoot   FileTransferConf                 `yaml:"file-transfer-root,omitempty"`
}

type APICreds struct {
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
}

// VPNConnConf includes configuration
// information for gRPC client on VPN service
type VPNConnConf struct {
	Endpoint string            `yaml:"endpoint"`
	Port     uint64            `yaml:"port"`
	AuthKey  string            `yaml:"auth-key"`
	SignKey  string            `yaml:"sign-key"`
	Dir      string            `yaml:"client-conf-dir"`
	CertConf CertificateConfig `yaml:"tls"`
}

type CertificateConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Directory string `yaml:"directory"`
	CertFile  string `yaml:"certfile"`
	CertKey   string `yaml:"certkey"`
	CAFile    string `yaml:"cafile"`
}

type ServiceConfig struct {
	Grpc       string            `yaml:"grpc,omitempty"`
	AuthKey    string            `yaml:"auth-key,omitempty"`
	SignKey    string            `yaml:"sign-key,omitempty"`
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

type FileTransferConf struct {
	Path string `yaml:"path"`
}
