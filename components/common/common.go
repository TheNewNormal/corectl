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

package common

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/TheNewNormal/corectl/components/common/assets"
	"github.com/TheNewNormal/corectl/components/host/session"
	"github.com/TheNewNormal/corectl/components/server"
	"github.com/TheNewNormal/corectl/release"
	"github.com/blang/semver"
	"github.com/helm/helm/log"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var RootCmdTmpl = &cobra.Command{
	Use:   session.AppName(),
	Short: release.ShortBanner,
	Long:  release.Banner,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func PScommand(cmd *cobra.Command, args []string) (err error) {
	var (
		cli   = session.Caller.CmdLine
		reply = &server.RPCreply{}
		srv   *release.Info
		pp    []byte
	)
	if srv, err = server.Daemon.Running(); err != nil {
		return
	}
	if reply, err = server.RPCQuery("ActiveVMs", &server.RPCquery{}); err != nil {
		return
	}
	running := reply.Running
	if cli.GetBool("json") {
		if pp, err = json.MarshalIndent(running, "", "    "); err == nil {
			fmt.Println(string(pp))
		}
		return
	}

	fmt.Println("\nServer:")
	srv.PrettyPrint(true)

	totalV, totalM, totalC := len(running), 0, 0
	for _, vm := range running {
		totalC, totalM = totalC+vm.Cpus, totalM+vm.Memory
	}
	fmt.Printf("Activity:\n Active VMs:\t%v\n "+
		"Total Memory:\t%v\n Total vCores:\t%v\n",
		totalV, totalM, totalC)
	for _, vm := range running {
		vm.PrettyPrint()
	}
	return
}

func versionCommand(cmd *cobra.Command, args []string) {
	var (
		latest          string
		running, remote semver.Version
		err             error
	)
	if session.AppName() != "corectld" {
		if srv, err := server.Daemon.Running(); err != nil {
			fmt.Printf("\nServer:\n Not running\n")
		} else {
			fmt.Println("\nServer:")
			srv.PrettyPrint(false)
			fmt.Println("\nClient:")
			session.Caller.Meta.PrettyPrint(false)
		}
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

func DefaultPreRunE(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("Incorrect usage. " +
			"This command doesn't accept any arguments.")
	}
	return nil
}

func STARTup(r *cobra.Command) (err error) {
	if session.Caller, err = session.New(); err != nil {
		return
	}
	session.Caller.CmdLine.BindPFlags(r.PersistentFlags())
	if session.AppName() != "corectld" {
		session.Caller.ServerAddress =
			session.Caller.CmdLine.GetString("server") + ":2511"
	}

	return r.Execute()
}

func InitTmpl(rootCmd *cobra.Command) {
	rootCmd.SetUsageTemplate(assets.Contents("cli/helpTemplate.tmpl"))
	rootCmd.PersistentFlags().BoolP("debug", "d", false,
		"adds additional verbosity, and options, directed at debugging "+
			"purposes and power users")
	var (
		utilsCmd = &cobra.Command{
			Use:   "utils",
			Short: "Some developer focused utilies",
			Run: func(cmd *cobra.Command, args []string) {
				cmd.Help()
			},
			Hidden: true,
		}
		manCmd = &cobra.Command{
			Use:     "genManPages",
			Short:   "Generates man pages",
			PreRunE: DefaultPreRunE,
			Run: func(cmd *cobra.Command, args []string) {
				header := &doc.GenManHeader{
					Title: session.AppName(), Source: " ",
				}
				doc.GenManTree(rootCmd, header,
					filepath.Join(session.ExecutableFolder(),
						"../documentation/man/"))
			},
		}
		mkdownCmd = &cobra.Command{
			Use:     "genMarkdownDocs",
			Short:   "Generates Markdown documentation",
			PreRunE: DefaultPreRunE,
			Run: func(cmd *cobra.Command, args []string) {
				doc.GenMarkdownTree(rootCmd,
					filepath.Join(session.ExecutableFolder(),
						"../documentation/markdown/"))
			},
		}
		versionCmd = &cobra.Command{
			Use:   "version",
			Short: "Shows version information",
			Run:   versionCommand,
		}
	)
	utilsCmd.AddCommand(manCmd, mkdownCmd)
	rootCmd.AddCommand(utilsCmd, versionCmd)
}
