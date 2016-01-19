// Copyright 2015 - António Meireles  <antonio.meireles@reformi.st>
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
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"github.com/spf13/cobra"
)

var (
	// RootCmd ...
	RootCmd = &cobra.Command{
		Use:   "corectl",
		Short: "CoreOS over OSX made simple.",
		Long: fmt.Sprintf("%s\n%s",
			"CoreOS over OSX made simple.",
			"❯❯❯ http://github.com/TheNewNormal/corectl"),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			versionCommand(cmd, args)
			return cmd.Usage()
		},
	}
	versionCmd = &cobra.Command{
		Use: "version", Short: "Shows corectl version information",
		Run: versionCommand,
	}
	engine sessionContext
	// -ldflags "-X main.Version=
	//            `git describe --abbrev=6 --dirty=-unreleased --always --tags`"
	Version, BuildDate string
)

func main() {
	if err := RootCmd.Execute(); err != nil {
		os.Exit(-1)
	}
}
func init() {
	// logger defaults
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
	log.SetPrefix("[corectl] ")

	RootCmd.PersistentFlags().Bool("debug", false,
		"adds extra verbosity, and options, for debugging purposes "+
			"and/or power users")

	RootCmd.SetUsageTemplate(HelpTemplate)
	RootCmd.AddCommand(versionCmd)

	// remaining defaults / startupChecks
	engine.init()
}

func versionCommand(cmd *cobra.Command, args []string) {
	var (
		err      error
		latest   *github.RepositoryRelease
		stamp, _ = time.Parse("2006-01-02T15:04:05MST", BuildDate)
	)
	fmt.Printf("%s\n%s\n%s\n\n", "CoreOS over OSX made simple.",
		" - http://github.com/TheNewNormal/corectl",
		"   Copyright (c) 2015-2016, António Meireles")
	fmt.Printf("Running version: %s\n"+
		"                 built with %s, %v\n",
		strings.TrimPrefix(Version, "v"), runtime.Version(), stamp)

	if latest, _, err =
		github.NewClient(nil).Repositories.GetLatestRelease("TheNewNormal",
			"corectl"); err != nil {
		return
	}
	fmt.Println("Latest upstream:", strings.TrimPrefix(
		strings.Trim(github.Stringify(latest.Name), "\""), "v"))
}
