// Copyright (c) 2016 by António Meireles  <antonio.meireles@reformi.st>.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package main

import (
	"fmt"
	"strings"

	"github.com/genevera/corectl/components/host/session"
	"github.com/genevera/corectl/components/server"
	"github.com/genevera/corectl/release"
	"github.com/blang/semver"
	"github.com/deis/pkg/log"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Shows version information",
	Run:   versionCommand,
}

func versionCommand(cmd *cobra.Command, args []string) {
	var (
		latest          string
		running, remote semver.Version
		err             error
	)
	if session.AppName() != "corectld" {
		if srv, err := server.Daemon.Running(); err == nil {
			fmt.Println("\nServer:")
			srv.PrettyPrint(false)
			fmt.Println("\nClient:")
		}
		session.Caller.Meta.PrettyPrint(false)
	} else {
		session.Caller.Meta.PrettyPrint(false)
	}

	if latest, err = release.LatestVersion(); err != nil {
		log.Debug("Skipped ustream version check: %s", err)
		return
	}
	running, err = semver.Parse(strings.TrimPrefix(release.Version, "v"))
	if err != nil {
		log.Debug("Local version %s is not well-formed", release.Version)
		return
	}

	remote, err = semver.Parse(strings.TrimPrefix(latest, "v"))
	if err != nil {
		log.Debug("Remote version %s is not well-formed", latest)
		return
	}

	if remote.NE(running) {
		fmt.Printf("--\n suggested version is %s\n", remote)
	}
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
