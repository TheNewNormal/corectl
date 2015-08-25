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

	"github.com/codegangsta/cli"
)

//
func deleteFlags() []cli.Flag {
	return []cli.Flag{
		cli.BoolFlag{
			Name:  "old",
			Usage: "removes outdated images.",
		},
	}
}

//
func deleteAction(c *cli.Context) {
	vm := &SessionContext.data
	vm.setChannel(c.String("channel"))
	vm.setVersion(c.String("version"))

	version := vm.Version
	local := getLocalImages()
	l := local[vm.Channel]
	fmt.Println(version, l)
	if local[vm.Channel].Len() > 0 {
		if c.String("old") == "true" {
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
}
