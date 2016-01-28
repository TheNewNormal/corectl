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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/TheNewNormal/corectl/uuid2ip"
	"github.com/TheNewNormal/libxhyve"
	"github.com/satori/go.uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	// until github.com/mitchellh/go-ps consumes it
	"github.com/yeonsh/go-ps"
)

var (
	runCmd = &cobra.Command{
		Use:     "run",
		Aliases: []string{"start"},
		Short:   "Starts a new CoreOS instance",
		PreRunE: func(cmd *cobra.Command, args []string) (err error) {
			if len(args) != 0 {
				return fmt.Errorf("Incorrect usage. " +
					"This command doesn't accept any arguments.")
			}
			engine.rawArgs.BindPFlags(cmd.Flags())

			return engine.allowedToRun()
		},
		RunE: runCommand,
	}
	xhyveCmd = &cobra.Command{
		Use:    "xhyve",
		Hidden: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 3 {
				return fmt.Errorf("Incorrect usage. " +
					"This command accepts exactly 3 arguments.")
			}
			return nil
		},
		RunE: xhyveCommand,
	}
)

func runCommand(cmd *cobra.Command, args []string) error {
	engine.VMs = append(engine.VMs, vmContext{})
	return engine.boot(0, engine.rawArgs)
}

func xhyveCommand(cmd *cobra.Command, args []string) (err error) {
	var (
		a0, a1, a2 string
		strDecode  = func(s string) (string, error) {
			b, e := base64.StdEncoding.DecodeString(s)
			return string(b), e
		}
	)

	if a0, err = strDecode(args[0]); err != nil {
		return err
	}
	if a1, err = strDecode(args[1]); err != nil {
		return err
	}
	if a2, err = strDecode(args[2]); err != nil {
		return err
	}
	return xhyve.Run(append(strings.Split(a0, " "),
		"-f", fmt.Sprintf("%s%v", a1, a2)), make(chan string))
}

func vmBootstrap(args *viper.Viper) (vm *VMInfo, err error) {
	vm = new(VMInfo)
	vm.publicIP = make(chan string)
	vm.errch, vm.done = make(chan error), make(chan bool)

	vm.PreferLocalImages = args.GetBool("local")
	vm.Detached = args.GetBool("detached")
	vm.Cpus = args.GetInt("cpus")
	vm.Extra = args.GetString("extra")
	vm.SSHkey = args.GetString("sshkey")
	vm.Root, vm.Pid = -1, -1

	vm.Name, vm.UUID = args.GetString("name"), args.GetString("uuid")

	if vm.UUID == "random" {
		vm.UUID = uuid.NewV4().String()
	} else if _, err = uuid.FromString(vm.UUID); err != nil {
		log.Printf("%s not a valid UUID as it doesn't follow RFC 4122. %s\n",
			vm.UUID, "    using a randomly generated one")
		vm.UUID = uuid.NewV4().String()
	}
	for {
		if vm.MacAddress, err = uuid2ip.GuestMACfromUUID(vm.UUID); err != nil {
			original := args.GetString("uuid")
			if original != "random" {
				log.Printf("unable to guess the MAC Address from the provided "+
					"UUID (%s). Using a randomly generated one one\n", original)
			}
			vm.UUID = uuid.NewV4().String()
		} else {
			break
		}
	}

	if vm.Name == "" {
		vm.Name = vm.UUID
	}

	if _, err = vmInfo(vm.Name); err == nil {
		if vm.Name == vm.UUID {
			return vm, fmt.Errorf("%s %s (%s)\n", "Aborting.",
				"Another VM is running with same UUID.", vm.UUID)
		}
		return vm, fmt.Errorf("%s %s (%s)\n", "Aborting.",
			"Another VM is running with same name.", vm.Name)
	}

	vm.Memory = args.GetInt("memory")
	if vm.Memory < 1024 {
		log.Printf("'%v' not a reasonable memory value. %s\n", vm.Memory,
			"Using '1024', the default")
		vm.Memory = 1024
	} else if vm.Memory > 8192 {
		log.Printf("'%v' not a reasonable memory value. %s %s\n", vm.Memory,
			"as presently we only support VMs with up to 8GB of RAM.",
			"setting it to '8192'")
		vm.Memory = 8192
	}

	if vm.Channel, vm.Version, err =
		lookupImage(normalizeChannelName(args.GetString("channel")),
			normalizeVersion(args.GetString("version")),
			false, vm.PreferLocalImages); err != nil {
		return
	}

	if err = vm.validateCDROM(args.GetString("cdrom")); err != nil {
		return
	}

	if err = vm.validateVolumes([]string{args.GetString("root")},
		true); err != nil {
		return
	}
	if err = vm.validateVolumes(pSlice(args.GetStringSlice("volume")),
		false); err != nil {
		return
	}

	vm.Ethernet = append(vm.Ethernet, NetworkInterface{Type: Raw})
	if err = vm.addTAPinterface(args.GetString("tap")); err != nil {
		return
	}

	err = vm.validateCloudConfig(args.GetString("cloud_config"))
	if err != nil {
		return
	}

	vm.InternalSSHprivKey, vm.InternalSSHauthKey, err = sshKeyGen()
	if err != nil {
		return vm, fmt.Errorf("%v (%v)",
			"Aborting: unable to generate internal SSH key pair (!)", err)
	}

	return vm, err
}

func (running *sessionContext) boot(slt int, rawArgs *viper.Viper) (err error) {
	var c = new(exec.Cmd)

	if running.VMs[slt].vm, err = vmBootstrap(rawArgs); err != nil {
		return
	}
	vm := running.VMs[slt].vm

	rundir := filepath.Join(running.runDir, vm.UUID)
	if err = os.RemoveAll(rundir); err != nil {
		return
	}
	if err = os.MkdirAll(rundir, 0755); err != nil {
		return
	}

	if err = nfsSetup(); err != nil {
		return
	}

	if c, err = vm.assembleBootPayload(); err != nil {
		return
	}
	vm.CreatedAt = time.Now()
	// saving now, in advance, without Pid to ensure {name,UUID,volumes}
	// atomicity
	if err = vm.storeConfig(); err != nil {
		return
	}

	go func() {
		timeout := time.After(30 * time.Second)
		select {
		case <-timeout:
			if p, ee := os.FindProcess(c.Process.Pid); ee == nil {
				p.Signal(os.Interrupt)
			}
			vm.errch <- fmt.Errorf("Unable to grab VM's IP after " +
				"30s (!)... Aborting")
		case ip := <-vm.publicIP:
			// afaict there's no race here, regardless of what `go build -race`
			// claims as vm.publicIP will only be triggered well after the
			// c.{Start,Run} calls...
			vm.Pid, vm.PublicIP = c.Process.Pid, ip
			if ee := vm.storeConfig(); ee != nil {
				vm.errch <- ee
			} else {
				if vm.Detached {
					log.Printf("started '%s' in background with IP %v and "+
						"PID %v\n", vm.Name, vm.PublicIP, c.Process.Pid)
				}
				close(vm.publicIP)
				close(vm.done)
			}
		}
	}()

	go func() {
		if !vm.Detached {
			c.Stdout, c.Stdin, c.Stderr = os.Stdout, os.Stdin, os.Stderr
			vm.errch <- c.Run()
		} else if ee := c.Start(); ee != nil {
			vm.errch <- ee
		} else {
			select {
			default:
				if ee := c.Wait(); ee != nil {
					log.Println(ee)
					vm.errch <- fmt.Errorf("VM exited with error " +
						"while attempting to start in background")
				}
			case <-vm.errch:
			}
		}
	}()

	for {
		select {
		case <-vm.done:
			if vm.Detached {
				return
			}
		case ee := <-vm.errch:
			return ee
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func runFlagsDefaults(setFlag *pflag.FlagSet) {
	setFlag.String("channel", "alpha", "CoreOS channel")
	setFlag.String("version", "latest", "CoreOS version")
	setFlag.String("uuid", "random", "VM's UUID")
	setFlag.Int("memory", 1024,
		"VM's RAM, in MB, per instance (1024 < memory < 8192)")
	setFlag.Int("cpus", 1, "VM's vCPUS")
	setFlag.String("cloud_config", "",
		"cloud-config file location (either a remote URL or a local path)")
	setFlag.String("sshkey", "", "VM's default ssh key")
	setFlag.String("root", "", "append a (persistent) root volume to VM")
	setFlag.String("cdrom", "", "append an CDROM (.iso) to VM")
	setFlag.StringSlice("volume", nil, "append disk volumes to VM")
	setFlag.String("tap", "", "append tap interface to VM")
	setFlag.BoolP("detached", "d", false,
		"starts the VM in detached (background) mode")
	setFlag.BoolP("local", "l", false,
		"consumes whatever image is `latest` locally instead of looking "+
			"online unless there's nothing available.")
	setFlag.StringP("name", "n", "",
		"names the VM. (if absent defaults to VM's UUID)")

	// available but hidden...
	setFlag.String("extra", "", "additional arguments to xhyve hypervisor")
	setFlag.MarkHidden("extra")
}

func init() {
	runFlagsDefaults(runCmd.Flags())
	RootCmd.AddCommand(runCmd)
	RootCmd.AddCommand(xhyveCmd)
}

func nfsSetup() (err error) {
	const exportsF = "/etc/exports"
	var (
		buf, bufN []byte
		shared    bool
		oldSig    = "/Users -network 192.168.64.0 " +
			"-mask 255.255.255.0 -alldirs -mapall="
		signature = fmt.Sprintf("%v -network %v -mask %v -alldirs "+
			"-mapall=%v:%v", engine.homedir, engine.network, engine.netmask,
			engine.uid, engine.gid)
		exportSet = func() (ok bool) {
			for _, line := range strings.Split(string(buf), "\n") {
				if strings.HasPrefix(line, signature) {
					ok = true
				}
				if !strings.HasPrefix(line, oldSig) {
					bufN = append(bufN, []byte(line+"\n")...)
				} else {
					bufN = append(bufN, []byte("\n")...)
				}
			}
			return
		}
		nfsIsRunning = func() bool {
			all, _ := ps.Processes()
			for _, p := range all {
				if strings.HasSuffix(p.Executable(), "nfsd") {
					return true
				}
			}
			return false
		}()
		exportsCheck = func(previous []byte) (err error) {
			var out []byte
			if out, err = exec.Command("nfsd", "-F",
				exportsF, "checkexports").Output(); err != nil {
				err = fmt.Errorf("unable to validate %s ('%v')", exportsF, out)
				// getting back to where we were
				ioutil.WriteFile(exportsF, previous, os.ModeAppend)
			}
			return
		}
	)
	// check if /etc/exports exists, and if not create an empty one
	if _, err = os.Stat(exportsF); os.IsNotExist(err) {
		if err = ioutil.WriteFile(exportsF, []byte(""), 0644); err != nil {
			return
		}
	}

	if buf, err = ioutil.ReadFile(exportsF); err != nil {
		return
	}

	if shared = exportSet(); !shared {
		if err = ioutil.WriteFile(exportsF, append(bufN,
			[]byte(signature+"\n")...), os.ModeAppend); err != nil {
			return
		}
	}

	if err = exportsCheck(buf); err != nil {
		return
	}

	if nfsIsRunning {
		if !shared {
			if err = exec.Command("nfsd", "update").Run(); err != nil {
				return fmt.Errorf("unable to update NFS "+
					"service definitions... (%v)", err)
			}
			log.Printf("'%s' was made available to VMs via NFS\n",
				engine.homedir)
		} else {
			log.Printf("'%s' was already available to VMs via NFS\n",
				engine.homedir)
		}
	} else {
		if err = exec.Command("nfsd", "start").Run(); err != nil {
			return fmt.Errorf("unable to start NFS service... (%v)", err)
		}
		log.Printf("NFS started in order for '%s' to be "+
			"made available to the VMs\n", engine.homedir)
	}
	return
}

func (vm *VMInfo) storeConfig() (err error) {
	rundir := filepath.Join(engine.runDir, vm.UUID)
	cfg, _ := json.MarshalIndent(vm, "", "    ")

	if engine.debug {
		fmt.Println(string(cfg))
	}

	if err = ioutil.WriteFile(fmt.Sprintf("%s/config", rundir),
		[]byte(cfg), 0644); err != nil {
		return
	}

	return normalizeOnDiskPermissions(rundir)
}

func (vm *VMInfo) assembleBootPayload() (cmd *exec.Cmd, err error) {
	var (
		cmdline = fmt.Sprintf("%s %s %s %s",
			"earlyprintk=serial", "console=ttyS0", "coreos.autologin",
			"uuid="+vm.UUID)
		prefix  = "coreos_production_pxe"
		vmlinuz = fmt.Sprintf("%s/%s/%s/%s.vmlinuz",
			engine.imageDir, vm.Channel, vm.Version, prefix)
		initrd = fmt.Sprintf("%s/%s/%s/%s_image.cpio.gz",
			engine.imageDir, vm.Channel, vm.Version, prefix)
		instr = []string{
			"libxhyve_bug",
			"-s", "0:0,hostbridge",
			"-l", "com1,stdio",
			"-s", "31,lpc",
			"-U", vm.UUID,
			"-m", fmt.Sprintf("%vM", vm.Memory),
			"-c", fmt.Sprintf("%v", vm.Cpus),
			"-A",
		}
		endpoint string
	)

	if vm.SSHkey != "" {
		cmdline = fmt.Sprintf("%s sshkey=\"%s\"", cmdline, vm.SSHkey)
	}

	if vm.Root != -1 {
		cmdline = fmt.Sprintf("%s root=/dev/vd%s", cmdline, string(vm.Root+'a'))
	}

	if endpoint, err = vm.metadataService(); err != nil {
		return
	}
	cmdline = fmt.Sprintf("%s endpoint=%s", cmdline, endpoint)

	if vm.CloudConfig != "" {
		if vm.CClocation == Local {
			cmdline = fmt.Sprintf("%s cloud-config-url=%s",
				cmdline, endpoint+"/cloud-config")
		} else {
			cmdline = fmt.Sprintf("%s cloud-config-url=%s",
				cmdline, vm.CloudConfig)
		}
	}

	if vm.Extra != "" {
		instr = append(instr, vm.Extra)
	}

	for v, vv := range vm.Ethernet {
		if vv.Type == Tap {
			instr = append(instr,
				"-s", fmt.Sprintf("2:%d,virtio-tap,%v", v, vv.Path))
		} else {
			instr = append(instr, "-s", fmt.Sprintf("2:%d,virtio-net", v))
		}
	}

	for _, v := range vm.Storage.CDDrives {
		instr = append(instr, "-s", fmt.Sprintf("3:%d,ahci-cd,%s",
			v.Slot, v.Path))
	}

	for _, v := range vm.Storage.HardDrives {
		instr = append(instr, "-s", fmt.Sprintf("4:%d,virtio-blk,%s",
			v.Slot, v.Path))
	}
	strEncode := func(s string) string {
		return base64.StdEncoding.EncodeToString([]byte(s))
	}
	return exec.Command(os.Args[0], "xhyve",
			strEncode(strings.Join(instr, " ")),
			strEncode(fmt.Sprintf("kexec,%s,%s,", vmlinuz, initrd)),
			strEncode(fmt.Sprintf("%v", cmdline))),
		err
}

func (vm *VMInfo) validateCloudConfig(config string) (err error) {
	if len(config) == 0 {
		return
	}

	var response *http.Response
	if response, err = http.Get(config); response != nil {
		response.Body.Close()
	}
	vm.CloudConfig = config
	if err == nil && (response.StatusCode == http.StatusOK ||
		response.StatusCode == http.StatusNoContent) {
		vm.CClocation = Remote
		return
	}
	if _, err = os.Stat(config); err != nil {
		return
	}
	vm.CloudConfig = filepath.Join(engine.pwd, config)
	vm.CClocation = Local
	return
}

func (vm *VMInfo) validateCDROM(path string) (err error) {
	if path == "" {
		return
	}
	var abs string
	if !strings.HasSuffix(path, ".iso") {
		return fmt.Errorf("Aborting: --cdrom payload MUST end in '.iso'"+
			" ('%s' doesn't)", path)
	}
	if _, err = os.Stat(path); err != nil {
		return err
	}
	if abs, err = filepath.Abs(path); err != nil {
		return
	}
	vm.Storage.CDDrives = make(map[string]StorageDevice, 0)
	vm.Storage.CDDrives["0"] = StorageDevice{
		Type: CDROM, Slot: 0, Path: abs,
	}
	return
}

func (vm *VMInfo) addTAPinterface(tap string) (err error) {
	if tap == "" {
		return
	}
	var dir, dev string
	if dir = filepath.Dir(tap); !strings.HasPrefix(dir, "/dev") {
		return fmt.Errorf("Aborting: '%v' not a valid tap device...", tap)
	}
	if dev = filepath.Base(tap); !strings.HasPrefix(dev, "tap") {
		return fmt.Errorf("Aborting: '%v' not a valid tap device...", tap)
	}
	if _, err = os.Stat(tap); err != nil {
		return
	}
	// check atomicity
	var up []VMInfo
	if up, err = allRunningInstances(); err != nil {
		return
	}
	for _, d := range up {
		for _, vv := range d.Ethernet {
			if dev == vv.Path {
				return fmt.Errorf("Aborting: %s already being used  "+
					"by another VM (%s)", dev,
					d.Name)
			}
		}
	}
	vm.Ethernet = append(vm.Ethernet, NetworkInterface{
		Type: Tap, Path: dev,
	})
	return
}

func (vm *VMInfo) validateVolumes(volumes []string, root bool) (err error) {
	var abs string
	for _, j := range volumes {
		if j != "" {
			if _, err = os.Stat(j); err != nil {
				return
			}
			if abs, err = filepath.Abs(j); err != nil {
				return
			}
			if !strings.HasSuffix(j, ".img") {
				return fmt.Errorf("Aborting: --volume payload MUST end"+
					" in '.img' ('%s' doesn't)", j)
			}
			// check atomicity
			var up []VMInfo
			if up, err = allRunningInstances(); err != nil {
				return
			}
			for _, d := range up {
				for _, vv := range d.Storage.HardDrives {
					if abs == vv.Path {
						return fmt.Errorf("Aborting: %s %s (%s)", abs,
							"already being used as a volume by another VM.",
							vv.Path)
					}
				}
			}

			if vm.Storage.HardDrives == nil {
				vm.Storage.HardDrives = make(map[string]StorageDevice, 0)
			}

			slot := len(vm.Storage.HardDrives)
			for _, z := range vm.Storage.HardDrives {
				if z.Path == abs {
					return fmt.Errorf("Aborting: attempting to set '%v' "+
						"as base of multiple volumes", j)
				}
			}
			vm.Storage.HardDrives[strconv.Itoa(slot)] = StorageDevice{
				Type: HDD, Slot: slot, Path: abs,
			}
			if root {
				vm.Root = slot
			}
		}
	}
	return
}
