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

	"github.com/genevera/corectl/components/host/session"
	"github.com/genevera/corectl/components/server"
	"github.com/deis/pkg/log"
	"github.com/spf13/cobra"
)

var (
	panicCmd = &cobra.Command{
		Use:     "panic [VMids]",
		Aliases: []string{"oops", "wtf"},
		Short:   "Hard kills a running CoreOS instance",
		Long: "Hard kills a running CoreOS instance.\nThis feature is " +
			"intended as a practical way to test and reproduce both cluster " +
			"failure scenarios and layouts resilient to them",
		PreRunE: func(cmd *cobra.Command, args []string) (err error) {
			session.Caller.CmdLine.BindPFlags(cmd.Flags())
			errMsg := fmt.Errorf("This command requires either " +
				"one argument to work or just '--random'.")
			if session.Caller.CmdLine.GetBool("random") {
				if len(args) != 0 {
					err = errMsg
				}
			} else if len(args) != 1 {
				err = errMsg
			}
			return
		},
		RunE: panicCommand,
	}
)

func panicCommand(cmd *cobra.Command, args []string) (err error) {
	var (
		in     server.RPCquery
		out    *server.RPCreply
		random = session.Caller.CmdLine.GetBool("random")
	)

	if _, err = server.Daemon.Running(); err != nil {
		return session.ErrServerUnreachable
	}

	if !random {
		in.Input = args
	}
	in.Forced = true
	if out, err = server.RPCQuery("StopVMs", &in); err != nil {
		return
	}
	if random {
		log.Info("'%v' gone", out.Output[0])
	}
	return
}

func init() {
	panicCmd.Flags().BoolP("random", "r", false,
		"hard kill a randomly choosen running instance")
	if session.AppName() != "corectld" {
		rootCmd.AddCommand(panicCmd)
	}
}
