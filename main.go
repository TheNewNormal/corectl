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
	"os"

	"github.com/codegangsta/cli"
)

func main() {
	SessionContext.init()
	blob := cli.NewApp()
	blob.HideVersion = true
	blob.Name = "coreos"
	blob.Version = "0.0.1"
	blob.Usage = fmt.Sprintf("%s\n%s",
		"CoreOS (on top of OS X and xhyve) made simple.",
		"            http://github.com/coreos/coreos-xhyve")
	blob.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "enables debug output",
		}, cli.BoolFlag{
			Name:  "json",
			Usage: "enables json output",
		},
	}
	blob.Commands = []cli.Command{
		pullCommand(),
		lsCommand(),
		rmCommand(),
		runCommand(),
		psCommand(),
		killCommand(),
	}
	blob.Run(os.Args)
}
