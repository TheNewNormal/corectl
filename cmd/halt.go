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
	"github.com/spf13/cobra"
)

var (
	killCmd = &cobra.Command{
		Use:     "kill [VMids]",
		Aliases: []string{"stop", "halt"},
		Short:   "Halts one or more running CoreOS instances",
		PreRunE: func(cmd *cobra.Command, args []string) (err error) {
			session.Caller.CmdLine.BindPFlags(cmd.Flags())
			errMsg := fmt.Errorf("This command requires either " +
				"one argument to work or just '--all'.")
			if session.Caller.CmdLine.GetBool("all") {
				if len(args) != 0 {
					err = errMsg
				}
			} else if len(args) != 1 {
				err = errMsg
			}
			return
		},
		RunE: killCommand,
	}
)

func killCommand(cmd *cobra.Command, args []string) (err error) {
	var in server.RPCquery

	if _, err = server.Daemon.Running(); err != nil {
		return session.ErrServerUnreachable
	}

	if !session.Caller.CmdLine.GetBool("all") {
		in.Input = args
	}
	_, err = server.RPCQuery("StopVMs", &in)
	return
}

func init() {
	killCmd.Flags().BoolP("all", "a", false, "halts all running instances")
	if session.AppName() != "corectld" {
		rootCmd.AddCommand(killCmd)
	}
}
