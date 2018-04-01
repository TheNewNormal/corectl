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
	"encoding/binary"
	"fmt"
	"strings"
	"syscall"

	"github.com/genevera/corectl/components/host/session"
	"github.com/genevera/corectl/components/server"
	"github.com/genevera/corectl/components/target/coreos"
	"github.com/deis/pkg/log"
	"github.com/satori/go.uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	runCmd = &cobra.Command{
		Use:     "run",
		Aliases: []string{"start"},
		Short:   "Boots a new CoreOS instance",
		RunE:    runCommand,
	}
)

func runCommand(cmd *cobra.Command, args []string) (err error) {
	var (
		vm  *server.VMInfo
		cli = session.Caller.CmdLine
	)

	if _, err = server.Daemon.Running(); err != nil {
		return session.ErrServerUnreachable
	}
	if vm, err = vmBootstrap(cli); err != nil {
		return
	}
	return bootIt(vm)
}

func bootIt(vm *server.VMInfo) (err error) {
	var reply = &server.RPCreply{}
	log.Info("'%v' boot logs can be found at '%v'", vm.Name, vm.Log())
	if reply, err =
		server.RPCQuery("Run", &server.RPCquery{VM: vm}); err != nil {
		return
	}
	log.Info("'%v' started successfuly with address %v and PID %v",
		reply.VM.Name, reply.VM.PublicIP, reply.VM.Pid)
	log.Info("'%v' console can be found at '%v'", reply.VM.Name, reply.VM.TTY())
	return
}

func vmBootstrap(args *viper.Viper) (vm *server.VMInfo, err error) {
	var (
		reply           = &server.RPCreply{}
		HostPhysicalMem uint64
		// only thing we need from github.com shirou/gopsutil
		getHwMemsize = func() (v uint64, err error) {
			var tStr string
			if tStr, err = syscall.Sysctl("hw.memsize"); err == nil {
				v = uint64(binary.LittleEndian.Uint64([]byte(tStr + "\x00")))
			}
			return
		}
	)
	vm = new(server.VMInfo)

	vm.OfflineMode = args.GetBool("offline")
	vm.Cpus = args.GetInt("cpus")
	vm.AddToHypervisor = args.GetString("extra")
	vm.AddToKernel = args.GetString("boot")
	vm.SSHkey = args.GetString("sshkey")
	vm.Pid = -1

	vm.Name = strings.ToLower(args.GetString("name"))
	vm.UUID = args.GetString("uuid")

	vm.Memory = args.GetInt("memory")
	if vm.Memory < 1024 {
		log.Warn("'%v' not a reasonable memory value. %s\n", vm.Memory,
			"Using '1024', the default")
		vm.Memory = 1024
	} else if vm.Memory > 8192 {
		log.Warn("'%v' not a reasonable memory value, as presently "+
			"we only support VMs with up to 8GB of RAM. setting "+
			"it to '8192'", vm.Memory)
		vm.Memory = 8192
	}

	if reply, err =
		server.RPCQuery("ActiveVMs", &server.RPCquery{}); err != nil {
		return
	}

	totalM := 0
	for _, v := range reply.Running {
		totalM = totalM + v.Memory
	}

	if HostPhysicalMem, err = getHwMemsize(); err != nil {
		return
	}

	if uint64(totalM+vm.Memory) > HostPhysicalMem/1024/1024*2/3 {
		err = fmt.Errorf("Aborted. Aggregated VMs memory usage would " +
			"become higher than 66pc of total host physical memory which " +
			"would lead to system unstability")
		return
	}

	if vm.UUID == "random" {
		vm.UUID = uuid.NewV4().String()
	} else if _, err = uuid.FromString(vm.UUID); err != nil {

		log.Warn("%s not a valid UUID as it doesn't follow RFC "+
			"4122. %s\n", vm.UUID, "Using a randomly generated one")
		vm.UUID = uuid.NewV4().String()
	}
	vm.UUID = strings.ToUpper(vm.UUID)

	reply, err = server.RPCQuery("UUIDtoMACaddr",
		&server.RPCquery{Input: []string{vm.UUID, args.GetString("uuid")}})
	if err != nil {
		return
	}
	vm.MacAddress, vm.UUID = reply.Output[0], reply.Output[1]

	vm.SharedHomedir = args.GetBool("shared-homedir")
	if vm.SharedHomedir == true {
		if reply, err = server.RPCQuery("HandlesNFS",
			&server.RPCquery{}); err != nil {
			return
		}
		if reply.WorkingNFS == false {
			log.Warn("NFS is not supported by the running 'corectld'")
			vm.SharedHomedir = false
		}
	}

	if vm.Name == "" {
		vm.Name = vm.UUID
	}

	vm.Channel = coreos.Channel(args.GetString("channel"))

	vm.Version = coreos.Version(args.GetString("version"))
	vm.Version, err =
		server.PullImage(vm.Channel, vm.Version, false, vm.OfflineMode)
	if err != nil {
		return
	}

	vm.ValidateCDROM("")
	if args.GetBool("format-root") {
		if args.GetString("root") == "" {
			err = fmt.Errorf("unable to format root volume as it wasn't set...")
			return
		}
		vm.FormatRoot = true
	}
	if err = vm.ValidateVolumes([]string{args.GetString("root")},
		true); err != nil {
		return
	}
	if err = vm.ValidateVolumes(
		viperStringSliceBugWorkaround(args.GetStringSlice("volume")),
		false); err != nil {
		return
	}

	vm.Ethernet =
		append(vm.Ethernet, server.NetworkInterface{Type: server.Raw})

	err = vm.ValidateCloudConfig(args.GetString("cloud_config"))
	if err != nil {
		return
	}

	if err = vm.SSHkeyGen(); err != nil {
		err = fmt.Errorf("Aborting: unable to generate internal SSH "+
			"key pair (!) (%v)", err)
	}

	return
}

func runFlagsDefaults(setFlag *pflag.FlagSet) {
	setFlag.StringP("channel", "c", "alpha", "CoreOS channel stream")
	setFlag.StringP("version", "v", "latest", "CoreOS version")
	setFlag.StringP("uuid", "u", "random", "VM's UUID")
	setFlag.IntP("memory", "m", 1024,
		"VM's RAM, in MB, per instance (1024 < memory < 8192)")
	setFlag.IntP("cpus", "N", 1, "VM number of virtual CPUs")
	setFlag.StringP("cloud_config", "L", "",
		"cloud-config file location (either an URL or a local path)")
	setFlag.StringP("sshkey", "k", "", "VM's default ssh key")
	setFlag.StringP("root", "r", "", "append a (persistent) root volume to VM")
	setFlag.BoolP("format-root", "F", false,
		"formats and partitions the (persistent) root volume")
	setFlag.StringSliceP("volume", "p", nil, "append disk volumes to VM")
	setFlag.BoolP("offline", "o", false,
		"doesn't go online to check for newer images than the "+
			"locally available ones unless there is none available.")
	setFlag.StringP("name", "n", "",
		"names the VM (default is VM's UUID)")
	setFlag.BoolP("shared-homedir", "H", false,
		"mounts (via NFS) host's homedir inside VM")
	setFlag.StringP("extra", "x", "", "additional arguments to the hypervisor")
	setFlag.StringP("boot", "b", "", "additional arguments to the kernel boot")
	// available but hidden...
	setFlag.StringP("tap", "t", "", "append tap interface to VM")
	setFlag.MarkHidden("tap")
	setFlag.MarkHidden("extra")
	setFlag.MarkHidden("boot")
}

func init() {
	runFlagsDefaults(runCmd.Flags())
	if session.AppName() != "corectld" {
		rootCmd.AddCommand(runCmd)
	}
}
