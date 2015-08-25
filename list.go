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

	"github.com/blang/semver"
	"github.com/codegangsta/cli"
)

func listAction(c *cli.Context) {

	SessionContext.data.setChannel(c.String("channel"))
	SessionContext.data.setVersion(c.String("version"))

	var channels []string
	if c.String("all") == "true" {
		channels = DefaultChannels
	} else {
		channels = append(channels, SessionContext.data.Channel)
	}
	local := getLocalImages()
	fmt.Println("locally available images")
	for _, i := range channels {
		fmt.Printf("  - %s channel \n", i)
		for _, d := range local[i] {
			fmt.Println("    -", d.String())
		}
	}
}

func listFlags() []cli.Flag {
	return []cli.Flag{
		cli.BoolFlag{
			Name:  "all,a",
			Usage: "lists all channels",
		},
	}
}

func getLocalImages() map[string]semver.Versions {
	local := make(map[string]semver.Versions, 0)
	for _, channel := range DefaultChannels {
		path := fmt.Sprintf("%s/images/%s",
			SessionContext.configDir, channel)
		files, _ := ioutil.ReadDir(path)
		var v semver.Versions
		for _, f := range files {
			if f.IsDir() {
				s, _ := semver.Make(f.Name())
				v = append(v, s)
			}
		}
		semver.Sort(v)
		local[channel] = v
	}
	return local
}
