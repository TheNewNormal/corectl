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
	"os"
	"os/user"
	"strings"

	"github.com/genevera/corectl/components/host/session"
	"github.com/genevera/corectl/release"
	"github.com/deis/pkg/log"
	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   session.AppName(),
		Short: release.ShortBanner,
		Long:  release.Banner,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.UsageFunc()(cmd)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	ErrNotEnoughRights = fmt.Errorf(
		"attempting a privileged operation without proper rights")
	ErrTooManyRights = fmt.Errorf("too many privileges invoking %v, "+
		"please call it as a regular user", session.AppName())
	ErrOwnerUnset = func(cmd *cobra.Command) error {
		return fmt.Errorf("attempting to call '%v' without "+
			"setting a valid user (--user)", cmd.Name())
	}
)

func init() {
	if session.AppName() != "corectld" {
		rootCmd.PersistentFlags().StringP("server", "s", "127.0.0.1",
			"corectld location")
		rootCmd.PersistentFlags().MarkHidden("server")
	}
	rootCmd.PersistentPreRunE =
		func(cmd *cobra.Command, args []string) (err error) {
			cli := session.Caller.CmdLine
			cli.BindPFlags(cmd.Flags())
			log.DefaultLogger.SetDebug(cli.GetBool("debug"))
			if session.AppName() != "corectld" {
				if session.Caller.Privileged {
					return ErrTooManyRights
				}
			} else {
				if !session.Caller.Privileged && cmd.Name() == "uuid2mac" {
					return ErrNotEnoughRights
				}
				if session.Caller.Privileged {
					if cmd.Name() == "uuid2mac" {
						return
					}
					if cmd.Name() == "start" {
						var usr *user.User
						usr, err = user.Lookup(cli.GetString("user"))
						if err != nil {
							return ErrOwnerUnset(cmd)
						}
						session.Caller.User = usr
					} else {
						return ErrTooManyRights
					}
				}
			}
			if !(strings.HasPrefix(cmd.Name(), "genM") ||
				cmd.Name() == "version") {
				if err = session.Caller.Network.SetContext(); err != nil {
					return
				}
				err = session.Caller.NormalizeOnDiskLayout()
			}
			return
		}
}

func defaultPreRunE(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("Incorrect usage. " +
			"This command doesn't accept any arguments.")
	}
	return nil
}

func viperStringSliceBugWorkaround(plain []string) []string {
	// getting around https://github.com/spf13/viper/issues/112
	var sliced []string
	for _, x := range plain {
		strip := strings.Replace(
			strings.Replace(x, "]", "", -1), "[", "", -1)
		for _, y := range strings.Split(strip, ",") {
			sliced = append(sliced, y)
		}
	}
	return sliced
}

func main() {
	var err error
	if session.Caller, err = session.New(); err != nil {
		return
	}
	session.Caller.CmdLine.BindPFlags(rootCmd.PersistentFlags())
	if session.AppName() != "corectld" {
		session.Caller.ServerAddress =
			session.Caller.CmdLine.GetString("server") + ":2511"
	}
	if err = rootCmd.Execute(); err != nil {
		log.Err(err.Error())
		os.Exit(-1)
	}
}
