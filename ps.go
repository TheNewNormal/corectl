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

	"github.com/codegangsta/cli"
)

func psFlags() []cli.Flag {
	return []cli.Flag{
		cli.BoolFlag{
			Name:  "all,a",
			Usage: "Shows expanded info about running instances",
		},
	}
}
func psAction(c *cli.Context) {
	ls, _ := ioutil.ReadDir(filepath.Join(SessionContext.configDir, "running"))
	for _, d := range ls {
		fmt.Printf("- %s (up %s)\n", d.Name(), time.Now().Sub(d.ModTime()))
		if got(c.Bool("a")) {
			cfg := filepath.Join(SessionContext.configDir,
				fmt.Sprintf("running/%s/config", d.Name()))
			cc, _ := ioutil.ReadFile(cfg)
			fmt.Printf("  %s\n", strings.Replace(string(cc), "\n", "\n  ", -1))
		}
	}
}
