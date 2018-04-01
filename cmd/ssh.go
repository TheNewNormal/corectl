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
	"fmt"
	"strings"

	"github.com/genevera/corectl/components/host/session"
	"github.com/genevera/corectl/components/server"
	"github.com/genevera/corectl/components/server/connector"
	"github.com/spf13/cobra"
)

var (
	sshCmd = &cobra.Command{
		Use:     "ssh VMid [\"command1;...\"]",
		Aliases: []string{"attach"},
		Short:   "Attach to or run commands inside a running CoreOS instance",
		PreRunE: func(cmd *cobra.Command, args []string) (err error) {
			session.Caller.CmdLine.BindPFlags(cmd.Flags())
			if len(args) < 1 {
				return fmt.Errorf("This command requires at least " +
					"one argument to work ")
			}
			return
		},
		RunE: sshCommand,
		Example: `  corectl ssh VMid                 // logins into VMid
  corectl ssh VMid "some commands" // runs 'some commands' inside VMid and exits`,
	}
	scpCmd = &cobra.Command{
		Use:     "put path/to/file VMid:/file/path/on/destination",
		Aliases: []string{"copy", "cp", "scp"},
		Short:   "copy file to inside VM",
		PreRunE: func(cmd *cobra.Command, args []string) (err error) {
			session.Caller.CmdLine.BindPFlags(cmd.Flags())
			if len(args) < 2 {
				return fmt.Errorf("This command requires at least " +
					"two argument to work ")
			}
			return
		},
		RunE: scpCommand,
		Example: `  // copies 'filePath' into '/destinationPath' inside VMid
  corectl put filePath VMid:/destinationPath`,
	}
)

func sshCommand(cmd *cobra.Command, args []string) (err error) {
	var (
		sshSession = &connector.SSHclient{}
		vm         *server.VMInfo
	)

	if vm, err = vmInfo(args[0]); err != nil {
		return
	}

	sshSession, err =
		connector.StartSSHsession(vm.PublicIP, vm.InternalSSHprivate)
	if err != nil {
		return
	}
	defer sshSession.Close()

	if len(args) > 1 {
		return sshSession.ExecuteRemoteCommand(strings.Join(args[1:], " "))
	}
	return sshSession.RemoteShell()
}

func vmInfo(id string) (vm *server.VMInfo, err error) {
	var reply = &server.RPCreply{}
	if _, err = server.Daemon.Running(); err != nil {
		err = session.ErrServerUnreachable
		return
	}

	if reply, err =
		server.RPCQuery("ActiveVMs", &server.RPCquery{}); err != nil {
		return
	}
	running := reply.Running
	for _, v := range running {
		if v.Name == id || v.UUID == id {
			return v, err
		}
	}
	return vm, fmt.Errorf("'%s' not found, or dead", id)
}

func scpCommand(cmd *cobra.Command, args []string) (err error) {
	var (
		session, vm                 = &connector.SSHclient{}, &server.VMInfo{}
		split                       = strings.Split(args[1], ":")
		source, destination, target = args[0], split[1], split[0]
	)
	if vm, err = vmInfo(target); err != nil {
		return
	}
	if session, err = connector.StartSSHsession(vm.PublicIP, vm.InternalSSHprivate); err != nil {
		return
	}
	defer session.Close()
	return session.SCoPy(source, destination, target)
}

func init() {
	if session.AppName() != "corectld" {
		rootCmd.AddCommand(sshCmd, scpCmd)
	}
}
