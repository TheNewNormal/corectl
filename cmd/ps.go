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
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/genevera/corectl/components/host/session"
	"github.com/genevera/corectl/components/server"
	"github.com/genevera/corectl/release"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

var (
	psCmd = &cobra.Command{
		Use:     "ps",
		Short:   "Lists running CoreOS instances",
		PreRunE: defaultPreRunE,
		RunE:    psCommand,
	}
	queryCmd = &cobra.Command{
		Use:     "query [VMids]",
		Aliases: []string{"q"},
		Short:   "Display information about the running CoreOS instances",
		PreRunE: func(cmd *cobra.Command, args []string) (err error) {
			session.Caller.CmdLine.BindPFlags(cmd.Flags())
			if (session.Caller.CmdLine.GetBool("ip") ||
				session.Caller.CmdLine.GetBool("tty") ||
				session.Caller.CmdLine.GetBool("online") ||
				session.Caller.CmdLine.GetBool("up") ||
				session.Caller.CmdLine.GetBool("uuid") ||
				session.Caller.CmdLine.GetBool("log")) && len(args) != 1 {
				err = fmt.Errorf("Incorrect Usage: only one argument " +
					"expected (a VM's name or UUID)")
			}
			return err
		},
		RunE: queryCommand,
	}
)

func psCommand(cmd *cobra.Command, args []string) (err error) {
	var (
		cli   = session.Caller.CmdLine
		reply = &server.RPCreply{}
		srv   *release.Info
		pp    []byte
	)
	if srv, err = server.Daemon.Running(); err != nil {
		return
	}
	if reply, err = server.RPCQuery("ActiveVMs", &server.RPCquery{}); err != nil {
		return
	}
	running := reply.Running
	if cli.GetBool("json") {
		if pp, err = json.MarshalIndent(running, "", "    "); err == nil {
			fmt.Println(string(pp))
		}
		return
	}

	fmt.Println("\nServer:")
	srv.PrettyPrint(true)

	totalV, totalM, totalC := len(running), 0, 0
	for _, vm := range running {
		totalC, totalM = totalC+vm.Cpus, totalM+vm.Memory
	}
	fmt.Printf("Activity:\n Active VMs:\t%v\n "+
		"Total Memory:\t%v\n Total vCores:\t%v\n",
		totalV, totalM, totalC)
	for _, vm := range running {
		vm.PrettyPrint()
	}
	return
}

func queryCommand(cmd *cobra.Command, args []string) (err error) {
	var (
		pp       []byte
		reply    = &server.RPCreply{}
		cli      = session.Caller.CmdLine
		selected map[string]*server.VMInfo
		vm       *server.VMInfo
		tabP     = func(selected map[string]*server.VMInfo) {
			w := new(tabwriter.Writer)
			w.Init(os.Stdout, 5, 0, 1, ' ', 0)
			fmt.Fprintf(w, "name\tchannel/version\tip\tonline\tcpu(s)\tram\t"+
				"uuid\tpid\tuptime\tvols\n")
			for _, vm := range selected {
				fmt.Fprintf(w, "%v\t%v/%v\t%v\t%t\t%v\t%v\t%v\t%v\t%v\t%v\n",
					vm.Name, vm.Channel, vm.Version, vm.PublicIP,
					vm.NotIsolated, vm.Cpus, vm.Memory, vm.UUID, vm.Pid,
					humanize.Time(vm.CreationTime), len(vm.Storage.HardDrives))
			}
			w.Flush()
		}
	)

	if _, err = server.Daemon.Running(); err != nil {
		return session.ErrServerUnreachable
	}

	if reply, err =
		server.RPCQuery("ActiveVMs", &server.RPCquery{}); err != nil {
		return
	}
	running := reply.Running

	if len(args) == 1 {
		if vm, err = vmInfo(args[0]); err != nil {
			if cli.GetBool("up") {
				fmt.Println(false)
				return nil
			}
			return
		}
		if cli.GetBool("ip") {
			fmt.Println(vm.PublicIP)
			return
		} else if cli.GetBool("uuid") {
			fmt.Println(vm.UUID)
			return
		} else if cli.GetBool("tty") {
			fmt.Println(vm.TTY())
			return
		} else if cli.GetBool("log") {
			fmt.Println(vm.Log())
			return
		} else if cli.GetBool("online") {
			fmt.Println(vm.NotIsolated)
			return
		} else if cli.GetBool("up") {
			fmt.Println(true)
			return
		}
	}

	if len(args) == 0 {
		selected = running
	} else {
		selected = make(map[string]*server.VMInfo)
		for _, target := range args {
			if vm, err = vmInfo(target); err != nil {
				return
			}
			selected[vm.UUID] = vm
		}
	}

	if cli.GetBool("json") {
		if pp, err = json.MarshalIndent(selected, "", "    "); err == nil {
			fmt.Println(string(pp))
		}
	} else if cli.GetBool("all") {
		tabP(selected)
	} else {
		for _, vm := range selected {
			fmt.Println(vm.Name)
		}
	}
	return
}

func init() {
	psCmd.Flags().BoolP("json", "j", false,
		"outputs in JSON for easy 3rd party integration")

	queryCmd.Flags().BoolP("json", "j", false,
		"outputs in JSON for easy 3rd party integration")
	queryCmd.Flags().BoolP("all", "a", false,
		"display a table with extended information about running "+
			"CoreOS instances")
	queryCmd.Flags().BoolP("ip", "i", false,
		"displays given instance IP address")
	queryCmd.Flags().BoolP("tty", "t", false,
		"displays given instance tty's location")
	queryCmd.Flags().BoolP("log", "l", false,
		"displays given instance boot logs location")
	queryCmd.Flags().BoolP("online", "o", false,
		"tells if at boot time VM had connectivity to outter world")
	queryCmd.Flags().BoolP("up", "u", false,
		"tells if a given VM is up or not")
	queryCmd.Flags().BoolP("uuid", "U", false,
		"returns VM's UUID")
	if session.AppName() != "corectld" {
		rootCmd.AddCommand(psCmd, queryCmd)
	}
}
