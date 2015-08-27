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

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	RootCmd = &cobra.Command{
		Use:   "coreos",
		Short: "CoreOS, on top of OS X and xhyve, made simple.",
		Long: fmt.Sprintf("%s\n%s",
			"CoreOS, on top of OS X and xhyve, made simple.",
			"❯❯❯ http://github.com/coreos/coreos-xhyve"),
		Run: func(cmd *cobra.Command, args []string) {
			versionCommand(cmd, args)
			cmd.Usage()
		},
	}
)

func init() {
	// viper & cobra
	viper.SetEnvPrefix("COREOS")
	viper.AutomaticEnv()

	RootCmd.Flags().Bool("json", false,
		"outputs in JSON for easy 3rd party integration")
	viper.BindPFlag("json", RootCmd.Flags().Lookup("json"))

	RootCmd.Flags().Bool("debug", false,
		"adds extra verbosity for debugging purposes")
	viper.BindPFlag("debug", RootCmd.Flags().Lookup("debug"))

	// logger defaults
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
	log.SetPrefix("[coreos] ")

	// remaining defaults / startupChecks
	SessionContext.init()
}
