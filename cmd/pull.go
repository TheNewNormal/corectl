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
	"github.com/genevera/corectl/components/host/session"
	"github.com/genevera/corectl/components/server"
	"github.com/genevera/corectl/components/target/coreos"
	"github.com/spf13/cobra"
)

var (
	pullCmd = &cobra.Command{
		Use:     "pull",
		Aliases: []string{"get", "fetch"},
		Short:   "Pulls a CoreOS image from upstream",
		PreRunE: defaultPreRunE,
		RunE:    pullCommand,
	}
)

func pullCommand(cmd *cobra.Command, args []string) (err error) {
	var (
		reply = &server.RPCreply{}
		cli   = session.Caller.CmdLine
		force = cli.GetBool("force")
	)

	if _, err = server.Daemon.Running(); err != nil {
		return session.ErrServerUnreachable
	}

	if cli.GetBool("warmup") {
		reply, err = server.RPCQuery("AvailableImages", &server.RPCquery{})
		if err != nil {
			return
		}
		local := reply.Images
		for _, channel := range coreos.Channels {
			if local[channel].Len() > 0 {
				if _, err =
					server.PullImage(channel, coreos.Version("latest"),
						force, false); err != nil {
					return
				}
			}
		}
		_, err = server.RPCQuery("AvailableImages", &server.RPCquery{})
		return
	}
	if _, err =
		server.PullImage(coreos.Channel(cli.GetString("channel")),
			coreos.Version(cli.GetString("version")),
			force, false); err != nil {
		return
	}
	_, err = server.RPCQuery("AvailableImages", &server.RPCquery{})
	return
}

func init() {
	pullCmd.Flags().StringP("channel", "c", "alpha", "CoreOS channel")
	pullCmd.Flags().StringP("version", "v", "latest", "CoreOS version")
	pullCmd.Flags().BoolP("force", "f", false,
		"forces the rebuild of an image, if already local")
	pullCmd.Flags().BoolP("warmup", "w", false,
		"ensures that all (populated) channels are on their latest versions")
	if session.AppName() != "corectld" {
		rootCmd.AddCommand(pullCmd)
	}
}
