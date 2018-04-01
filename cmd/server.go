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

	"github.com/genevera/corectl/components/host/darwin/misc/uuid2ip"
	"github.com/genevera/corectl/components/host/session"
	"github.com/genevera/corectl/components/server"
	"github.com/genevera/corectl/release"
	"github.com/deis/pkg/log"
	"github.com/satori/go.uuid"

	"github.com/everdev/mack"
	"github.com/spf13/cobra"
)

var (
	serverStartCmd = &cobra.Command{
		Use:   "start",
		Short: "Starts corectld",
		RunE:  serverStartCommand,
	}
	shutdownCmd = &cobra.Command{
		Use:     "stop",
		Aliases: []string{"shutdown"},
		Short:   "Stops corectld",
		RunE:    shutdownCommand,
	}
	statusCmd = &cobra.Command{
		Use:   "status",
		Short: "Shows corectld status",
		RunE:  psCommand,
	}
	uuidToMacCmd = &cobra.Command{
		Use: "uuid2mac",
		Short: "returns the MAC address that will assembled from the " +
			"given UUID",
		RunE:   uuidToMacCommand,
		Hidden: true,
	}
)

func uuidToMacCommand(cmd *cobra.Command, args []string) (err error) {
	var macAddr string
	if _, err = uuid.FromString(args[0]); err != nil {
		log.Warn("%s not a valid UUID as it doesn't follow RFC "+
			"4122", args[0])
		// given that we only call this with dats generated with
		// uuid.NewV4().String() ...
		err = fmt.Errorf("Something went very wrong, as we're unable to "+
			"generate a MAC address from the provided UUID (%s). Please fill "+
			"a bug at https://github.com/genevera/corectl/issues with "+
			"this error and wait there for our feedback...", args[0])
	} else if macAddr, err = uuid2ip.GuestMACfromUUID(args[0]); err == nil {
		fmt.Println(macAddr)
	}
	return
}

func shutdownCommand(cmd *cobra.Command, args []string) (err error) {
	if _, err = server.Daemon.Running(); err != nil {
		return
	}
	_, err = server.RPCQuery("Stop", &server.RPCquery{})
	return
}

func serverStartCommand(cmd *cobra.Command, args []string) (err error) {
	var (
		srv    *release.Info
		cli    = session.Caller.CmdLine
		bugfix = viperStringSliceBugWorkaround
	)

	if srv, err = server.Daemon.Running(); err == nil {
		return fmt.Errorf("corectld already started (with pid %v)",
			srv.Pid)
	}

	if !session.Caller.Privileged {
		if err = mack.Tell("System Events",
			"do shell script \""+session.Executable()+" start "+
				" -u "+session.Caller.Username+
				" -D "+cli.GetString("domain")+
				" --dns-port "+cli.GetString("dns-port")+
				" -r "+strings.Join(bugfix(
				cli.GetStringSlice("recursive-nameservers")), ",")+
				" > /dev/null 2>&1 & \" with administrator privileges",
			"delay 3"); err != nil {
			return
		}
		if srv, err = server.Daemon.Running(); err != nil {
			return err
		}
		fmt.Println("Started corectld:")
		srv.PrettyPrint(true)
		return
	}
	server.LocalDomainName = cli.GetString("domain")
	server.EmbeddedDNSport = cli.GetString("dns-port")
	server.RecursiveNameServers =
		bugfix(cli.GetStringSlice("recursive-nameservers"))
	server.Daemon = server.New()

	return server.Start()
}

func init() {
	if session.AppName() == "corectld" {
		serverStartCmd.Flags().StringP("user", "u", "",
			"sets the user that will 'own' the corectld instance")
		serverStartCmd.Flags().StringP("domain", "D", server.LocalDomainName,
			"sets the dns domain under which the created VMs will operate")
		serverStartCmd.Flags().StringSliceP("recursive-nameservers", "r",
			server.RecursiveNameServers, "coma separated list of the recursive "+
				"nameservers to be used by the embedded dns server")
		serverStartCmd.Flags().String("dns-port", "15353",
			"embedded dns server port")
		rootCmd.AddCommand(shutdownCmd, statusCmd,
			serverStartCmd, uuidToMacCmd)
	}
}
