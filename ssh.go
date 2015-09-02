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
	"log"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	sshCmd = &cobra.Command{
		Use:     "ssh",
		Aliases: []string{"attach"},
		Short:   "attach to a running VM",
		Run:     sshCommand,
	}
)

func sshCommand(cmd *cobra.Command, args []string) {
	if vm, err := findVM(args[0]); err != nil {
		log.Fatalln(err)
	} else {
		if len(args) == 1 {
			vm.attach()
		} else {
			// FIXME ...
			fmt.Println("TBD")
			// vm.runCommand(args[1:])
		}
	}
}

func findVM(id string) (vm VMInfo, err error) {
	ls, _ := ioutil.ReadDir(filepath.Join(SessionContext.configDir,
		"running"))
	if len(ls) > 0 {
		for _, d := range ls {
			vm, err := getSavedConfig(d.Name())
			if err == nil && vm.isAlive() {
				if vm.Name == id || vm.UUID == id {
					return vm, err
				}
			}
		}
	}
	return vm, fmt.Errorf("'%s' not found, or dead", id)
}

func init() {
	RootCmd.AddCommand(sshCmd)
}
