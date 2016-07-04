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
	"os/user"

	"github.com/TheNewNormal/corectl/components/common"
	"github.com/TheNewNormal/corectl/components/host/session"

	"github.com/helm/helm/log"
	"github.com/spf13/cobra"
)

var rootCmd = common.RootCmdTmpl

func init() {
	rootCmd.PersistentPreRunE =
		func(cmd *cobra.Command, args []string) (err error) {
			cli := session.Caller.CmdLine
			cli.BindPFlags(cmd.Flags())
			if cli.GetBool("debug") {
				log.IsDebugging = true
			}

			if session.Caller.Privileged {
				if cmd.Name() == "start" {
					var usr *user.User
					usr, err = user.Lookup(cli.GetString("user"))
					if err != nil {
						err = fmt.Errorf("attempting to call '%v' without "+
							"setting a valid user (--user)", cmd.Name())
						return
					}
					session.Caller.User = usr
				} else {
					return fmt.Errorf("too many privileges invoking corectl. " +
						"running directly as root, or via 'sudo', only " +
						"tolerated with 'corectld server start'")
				}
			}
			return session.Caller.NormalizeOnDiskLayout()
		}
	common.InitTmpl(rootCmd)
}

func main() {
	if err := common.STARTup(rootCmd); err != nil {
		log.Die(err.Error())
	}
}
