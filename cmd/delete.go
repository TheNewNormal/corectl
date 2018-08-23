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
	"github.com/deis/pkg/log"

	"github.com/genevera/corectl/components/host/session"
	"github.com/genevera/corectl/components/server"
	"github.com/genevera/corectl/components/target/coreos"
	"github.com/spf13/cobra"
)

var (
	rmCmd = &cobra.Command{
		Use:     "rm",
		Aliases: []string{"rmi"},
		Short:   "Remove(s) CoreOS image(s) from the local filesystem",
		PreRunE: defaultPreRunE,
		RunE:    rmCommand,
	}
)

func rmCommand(cmd *cobra.Command, args []string) (err error) {
	var (
		cli     = session.Caller.CmdLine
		channel = coreos.Channel(cli.GetString("channel"))
		version = coreos.Version(cli.GetString("version"))
		reply   = &server.RPCreply{}
	)
	if _, err = server.Daemon.Running(); err != nil {
		return session.ErrServerUnreachable
	}

	if reply, err = server.RPCQuery("AvailableImages", &server.RPCquery{}); err != nil {
		return
	}
	local := reply.Images

	l := local[channel]
	if cli.GetBool("old") {
		for _, v := range l[0 : l.Len()-1] {
			if _, err = server.RPCQuery("RemoveImage", &server.RPCquery{
				Input: []string{channel, v.String()}}); err != nil {
				return
			}
			log.Info("removed %s/%s", channel, v.String())
		}
		return
	}

	if version == "latest" {
		if l.Len() > 0 {
			version = l[l.Len()-1].String()
		} else {
			log.Warn("nothing to delete")
			return
		}
	}

	if _, err = server.RPCQuery("RemoveImage", &server.RPCquery{
		Input: []string{channel, version}}); err != nil {
		return
	}

	log.Info("removed %s/%s\n", channel, version)

	return
}

func init() {
	rmCmd.Flags().StringP("channel", "c", "alpha", "CoreOS channel")
	rmCmd.Flags().StringP("version", "v", "latest", "CoreOS version")
	rmCmd.Flags().BoolP("purge", "p", false, "purges outdated images")
	if session.AppName() != "corectld" {
		rootCmd.AddCommand(rmCmd)
	}
}
