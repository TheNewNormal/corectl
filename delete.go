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
	rmCmd = &cobra.Command{
		Use:     "rm",
		Aliases: []string{"rmi"},
		Short:   "deletes CoreOS image locally",
		Run:     rmCommand,
	}
)

func rmCommand(cmd *cobra.Command, args []string) {
	viper.BindPFlags(cmd.Flags())

	vm := &SessionContext.data[0]

	vm.setChannel(viper.GetString("channel"))
	vm.setVersion(viper.GetString("version"))

	version := vm.Version
	local := getLocalImages()
	l := local[vm.Channel]
	if l.Len() == 0 {
		return
	}
	if viper.GetBool("old") {
		for _, i := range l[0 : l.Len()-1] {
			if err := os.RemoveAll(fmt.Sprintf("%s/images/%s/%s",
				SessionContext.configDir,
				vm.Channel, i)); err != nil {
				log.Fatalln(err)
			}
		}
	} else {
		if version == "latest" {
			version = l[l.Len()-1].String()
		}
		if err := os.RemoveAll(fmt.Sprintf("%s/images/%s/%s",
			SessionContext.configDir,
			vm.Channel, version)); err != nil {
			log.Fatalln(err)
		}
	}
}

func init() {
	rmCmd.Flags().String("channel", "alpha", "CoreOS channel")
	rmCmd.Flags().String("version", "latest", "CoreOS version")
	rmCmd.Flags().Bool("old", false,
		"removes outdated images")

	RootCmd.AddCommand(rmCmd)
}
