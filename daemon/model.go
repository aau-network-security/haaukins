// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package daemon

import (
	dockerclient "github.com/fsouza/go-dockerclient"
)

const (
	notfoundpage = `
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>404 Not Found &mdash; National Training Platform</title>
  <style>
   .h {
       font-family: -apple-system,BlinkMacSystemFont,avenir next,avenir,helvetica neue,helvetica,ubuntu,roboto,noto,segoe ui,arial,sans-serif;
   }

   .w {
       color: white;
   }

   .aau-logo {
       margin-top: 80px;
       height: 160px;
       width: 156px;
	background-image: url(data:image/png;base64,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);
}
  </style>
</head>
<body style="margin: 0">
    <div style="margin: 0; width: 100%; padding: 10px 0; background: #211a52;">
	<center>
	    <h2 class="h w">National Training Platform</h2>
	</center>
    </div>
    <center>
	<h1 class="h">
	    404 Not Found
	</h1>

	<p class="h">Did you look for an event? Please contact your instructor for a direct link.</p>
	<div class="aau-logo">
	</div>
    </center>

</body>
</html>`
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
	DBServer 		   string 							`yaml:"db-server,omitempty"`
	AuthKey 		   string 							`yaml:"db-auth-key,omitempty"`
	SignKey 		   string 							`yaml:"db-sign-key,omitempty"`
	UsersFile          string                           `yaml:"users-file,omitempty"`
	ExercisesFile      string                           `yaml:"exercises-file,omitempty"`
	FrontendsFile      string                           `yaml:"frontends-file,omitempty"`
	OvaDir             string                           `yaml:"ova-directory,omitempty"`
	LogDir             string                           `yaml:"log-directory,omitempty"`
	EventsDir          string                           `yaml:"events-directory,omitempty"`
	DockerRepositories []dockerclient.AuthConfiguration `yaml:"docker-repositories,omitempty"`
	SigningKey         string                           `yaml:"sign-key,omitempty"`
	TLS                struct {
		Enabled   bool   `yaml:"enabled"`
		Directory string `yaml:"directory"`
		CertFile string `yaml:"certfile"`
		CertKey string `yaml:"certkey"`
	} `yaml:"tls,omitempty"`
}




