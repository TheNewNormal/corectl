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
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	psCmd = &cobra.Command{
		Use:   "ps",
		Short: "lists running CoreOS instances",
		Run:   psCommand,
	}
)

func psCommand(cmd *cobra.Command, args []string) {
	ls, _ := ioutil.ReadDir(filepath.Join(SessionContext.configDir, "running"))
	if len(ls) > 0 {
		for _, d := range ls {
			fmt.Printf("- %s (up %s)\n", d.Name(), time.Now().Sub(d.ModTime()))
			if buf, _ := ioutil.ReadFile(filepath.Join(SessionContext.configDir,
				fmt.Sprintf("running/%s/%s", d.Name(), "ip"))); buf != nil {
				fmt.Println("  - IP:", string(buf))
			}
			if viper.GetBool("ps.a") {
				cfg := filepath.Join(SessionContext.configDir,
					fmt.Sprintf("running/%s/config", d.Name()))
				cc, _ := ioutil.ReadFile(cfg)
				fmt.Printf("  %s\n", strings.Replace(string(cc), "\n", "\n  ", -1))
			}
		}
	}
}

func init() {
	psCmd.Flags().BoolP("all", "a", false,
		"shows extended info about running instances")
	viper.BindPFlag("ps.a", psCmd.Flags().Lookup("all"))

	RootCmd.AddCommand(psCmd)
}
