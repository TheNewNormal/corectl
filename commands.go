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

import "github.com/codegangsta/cli"

func pullCommand() cli.Command {
	return cli.Command{
		Name:    "pull",
		Usage:   "Pull a CoreOS image from upstream",
		Aliases: []string{"get", "fetch"},
		Flags:   imageFlags(),
		Action:  pullAction,
	}
}

func runCommand() cli.Command {
	return cli.Command{
		Name:  "run",
		Usage: "Runs a new CoreOS container",
		Flags: append(append([]cli.Flag(nil),
			runFlags()...), imageFlags()...),
		Action: runAction,
	}
}

func lsCommand() cli.Command {
	return cli.Command{
		Name:    "ls",
		Aliases: []string{"list"},
		Usage:   "Lists locally available CoreOS images",
		Flags: append(append([]cli.Flag(nil),
			listFlags()...), imageFlags()...),
		Action: listAction,
	}
}

func psCommand() cli.Command {
	return cli.Command{
		Name:   "ps",
		Usage:  "Lists running CoreOS instances",
		Action: psAction,
		Flags:  psFlags(),
	}
}

func rmCommand() cli.Command {
	return cli.Command{
		Name:    "rm",
		Aliases: []string{"rmi"},
		Usage:   "Deletes CoreOS image locally",
		Flags: append(append([]cli.Flag(nil),
			deleteFlags()...), imageFlags()...),
		Action: deleteAction,
	}
}

func killCommand() cli.Command {
	return cli.Command{
		Name:   "kill",
		Usage:  "Kills a running CoreOS instance",
		Action: NOTyetImplementedCommand,
	}
}
