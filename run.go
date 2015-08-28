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
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/satori/go.uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	runCmd = &cobra.Command{
		Use:     "run",
		Aliases: []string{"start"},
		Short:   "runs a new CoreOS container",
		Run:     runCommand,
	}
)

func runCommand(cmd *cobra.Command, args []string) {
	SessionContext.canRun()
	vm := &SessionContext.data

	vm.setChannel(viper.GetString("channel"))
	vm.setVersion(viper.GetString("version"))

	vm.lookupImage()

	vm.xhyveCheck(viper.GetString("xhyve"))
	vm.tweakXhyve(viper.GetString("extra"))

	vm.uuidCheck(viper.GetString("uuid"))
	vm.validateCPU(viper.GetString("cpus"))
	vm.validateRAM(viper.GetString("memory"))
	vm.setSSHKey(viper.GetString("sshkey"))

	vm.validateCDROM(viper.GetString("cdrom"))
	vm.validateVolumes(strings.Fields(viper.GetString("root")), true)
	vm.validateVolumes(pSlice("volume"), false)
	vm.validateNetworkInterfaces(pSlice("net"))
	vm.validateCloudConfig(viper.GetString("cloud_config"))

	username, _ := user.LookupId(SessionContext.uid)
	cmdline := fmt.Sprintf("%s %s %s %s %s", "earlyprintk=serial",
		"console=ttyS0", "coreos.autologin",
		"localuser="+username.Username, "uuid="+vm.UUID)
	if vm.SSHkey != "" {
		cmdline = fmt.Sprintf("%s sshkey=\"%s\"", cmdline, vm.SSHkey)
	}
	if vm.Root != "" {
		t, _ := strconv.Atoi(vm.Root)
		cmdline = fmt.Sprintf("%s root=/dev/vd%s", cmdline,
			string(t+'a'))
	}
	vmlinuz := fmt.Sprintf("%s/images/%s/%s/coreos_production_pxe.vmlinuz",
		SessionContext.configDir, vm.Channel, vm.Version)
	initrd := fmt.Sprintf("%s/images/%s/%s/coreos_production_pxe_image.cpio.gz",
		SessionContext.configDir, vm.Channel, vm.Version)

	instr := []string{
		"-s", "0:0,hostbridge",
		"-l", "com1,stdio",
		"-s", "31,lpc",
		"-U", vm.UUID,
		"-m", fmt.Sprintf("%sM", vm.Memory),
		"-c", vm.Cpus,
		"-A",
	}
	if vm.Extra != "" {
		instr = append(instr, vm.Extra)
	}
	rundir := fmt.Sprintf("%s/running/%s/", SessionContext.configDir, vm.UUID)
	if _, err := os.Stat(filepath.Join(rundir, "/config")); err == nil {
		log.Fatalln("Aborting. Another VM seems to be running with same UUID.")
	}
	if err := os.MkdirAll(rundir, 0755); err != nil {
		log.Fatalln("unable to create", rundir)
	}
	if vm.CloudConfig != "" {
		if vm.CClocation == Local {
			cc, _ := ioutil.ReadFile(vm.CloudConfig)
			if err := ioutil.WriteFile(
				fmt.Sprintf("%s/cloud-config.local", rundir),
				cc, 0644); err != nil {
				log.Fatalln(err)
			}
		} else {
			cmdline = fmt.Sprintf("%s cloud-config-url=%s",
				cmdline, vm.CloudConfig)
		}
	}
	vm.setDefaultNIC()
	for _, v := range vm.Network.Raw {
		instr = append(instr, "-s", fmt.Sprintf("2:%d,virtio-net", v.Slot))
	}

	// for _, v := range vm.Network.Tap {
	// 	instr = append(instr, "-s", fmt.Sprintf("2:%d,virtio-tap,%s", v.Slot))
	// }

	for _, v := range vm.Storage.CDDrives {
		instr = append(instr, "-s", fmt.Sprintf("3:%d,ahci-cd,%s",
			v.Slot, v.Path))
	}
	for _, v := range vm.Storage.HardDrives {
		instr = append(instr, "-s", fmt.Sprintf("4:%d,virtio-blk,%s",
			v.Slot, v.Path))
	}

	usersDir := etcExports{}
	usersDir.share()

	cfg, _ := json.MarshalIndent(vm, "", "    ")
	if SessionContext.debug {
		fmt.Println(string(cfg))
	}
	if err := ioutil.WriteFile(fmt.Sprintf("%s/config", rundir),
		[]byte(cfg), 0644); err != nil {
		log.Fatalln(err)
	}

	if SessionContext.hasPowers {
		if err := fixPerms(rundir); err != nil {
			log.Fatalln(err)
		}
	}

	defer func() {
		if err := os.RemoveAll(rundir); err != nil {
			log.Fatalln(err)
		}
	}()

	fmt.Println("\nbooting ...")
	c := exec.Command(vm.Xhyve, append(instr, "-f",
		fmt.Sprintf("kexec,%s,%s,%s", vmlinuz, initrd, cmdline))...)

	c.Stdout, c.Stdin, c.Stderr = os.Stdout, os.Stdin, os.Stderr

	if err := c.Run(); err != nil {
		log.Println("xhyve exited with", err)
	}

	usersDir.unshare()
}

func init() {
	runCmd.Flags().String("channel", "alpha", "CoreOS channel")
	runCmd.Flags().String("version", "latest", "CoreOS version")
	runCmd.Flags().String("uuid", "random", "VM's UUID")
	runCmd.Flags().String("memory", "1024", "VM's RAM")
	runCmd.Flags().String("cpus", "1", "VM's vCPUS")
	runCmd.Flags().String("cloud_config", "",
		"cloud-config file location (either URL or local path)")
	runCmd.Flags().String("sshkey", "", "VM's default ssh key")
	runCmd.Flags().String("xhyve", "/usr/local/bin/xhyve",
		"xhyve binary to use")
	if SessionContext.debug {
		runCmd.Flags().String("extra", "",
			"additional arguments to xhyve hypervisor")
	}
	runCmd.Flags().String("root", "",
		"append a (persistent) root volume to VM")
	runCmd.Flags().String("cdrom", "",
		"append an CDROM (.iso) to VM")
	// runCmd.Flags().String("tap", "",
	// 	"append a tap interface to VM")

	runCmd.Flags().StringSlice("volume", nil,
		"append disk volumes to VM")
	runCmd.Flags().StringSlice("net", nil,
		"append additional network interfaces to VM")

	// Thanks God, for the for loop!
	for _, v := range []string{"channel", "version", "uuid",
		"memory", "cpus", "cloud_config", "sshkey", "xhyve",
		"extra", "root", "cdrom", "volume", "net"} {
		viper.BindPFlag(v, runCmd.Flags().Lookup(v))
	}

	RootCmd.AddCommand(runCmd)

}

type etcExports struct {
	restart   bool
	shared    bool
	exports   string
	signature string
	buf       []byte
}

func (f *etcExports) init() {
	f.exports = "/etc/exports"
	var err error
	f.buf, err = ioutil.ReadFile(f.exports)
	if err != nil {
		log.Fatalln(err)
	}
	f.signature = fmt.Sprintf("/Users %s -alldirs -mapall=%s:%s",
		"-network 192.168.64.0 -mask 255.255.255.0",
		SessionContext.uid, SessionContext.gid)
	f.restart, f.shared = false, false
	lines := strings.Split(string(f.buf), "\n")

	for _, lc := range lines {
		if lc == f.signature {
			f.shared = true
			break
		}
	}
}
func (f *etcExports) reload() {
	cmd := exec.Command("nfsd", "restart")
	if err := cmd.Run(); err != nil {
		log.Fatalln("unable to restart NFS...", err)
	}
}

func (f *etcExports) share() {
	f.init()
	if !f.shared {
		ioutil.WriteFile(f.exports,
			append(f.buf, append([]byte("\n"),
				append([]byte(f.signature), []byte("\n")...)...)...),
			os.ModeAppend)
		f.reload()
	}
}
func (f *etcExports) unshare() {
	f.init()
	if f.shared {
		ioutil.WriteFile(f.exports, bytes.Replace(f.buf,
			append(append([]byte("\n"), []byte(f.signature)...),
				[]byte("\n")...), []byte(""), -1), os.ModeAppend)
		f.reload()
	}
}

func (vm *VMInfo) xhyveCheck(xhyve string) {
	vm.Xhyve = xhyve
	if _, err := exec.LookPath(xhyve); err != nil {
		log.Fatalln(err)
	}
}
func (vm *VMInfo) uuidCheck(xxid string) {
	if xxid == "random" {
		vm.UUID = uuid.NewV4().String()
	} else {
		if _, err := uuid.FromString(xxid); err != nil {
			log.Printf("%s not a valid UUID as it doesn't follow RFC 4122. %s",
				xxid, "    using a randomly generated one")
			vm.UUID = uuid.NewV4().String()
		} else {
			vm.UUID = xxid
		}
	}
}

func (vm *VMInfo) validateCPU(cores string) {
	if _, err := strconv.Atoi(cores); err != nil {
		log.Printf(" %s not a reasonable CPU #. %s", cores,
			"    using '1', the default")
		cores = "1"
	}
	vm.Cpus = cores
}

func (vm *VMInfo) validateRAM(ram string) {
	if v, err := strconv.Atoi(ram); err != nil || v < 1024 {
		fmt.Printf(" '%s' not a reasonable memory value. %s", ram,
			"Using '1024', the default")
		ram = "1024"
	} else if v > 3072 {
		fmt.Printf(" '%s' not a reasonable memory value. %s %s", ram,
			"as presently xhyve only supports VMs with up to 3GB of RAM.",
			"setting it to '3072'")
		ram = "3072"
	} else {
		vm.Memory = ram
	}
}
func (vm *VMInfo) validateCloudConfig(config string) {
	if config != "" {
		response, err := http.Get(config)
		if response != nil {
			response.Body.Close()
		}
		vm.CloudConfig = config
		if err == nil && response.StatusCode == 200 {
			vm.CClocation = Remote
		} else {
			if _, err := os.Stat(config); err != nil {
				log.Fatalln(err)
			}
			vm.CloudConfig = filepath.Join(SessionContext.pwd, config)
			vm.CClocation = Local
		}
	}
}
func (vm *VMInfo) setSSHKey(key string) {
	if key != "" {
		vm.SSHkey = key
	}
}

func (vm *VMInfo) tweakXhyve(extra string) {
	vm.Extra = extra
}

func (vm *VMInfo) validateCDROM(path string) {
	if path != "" {
		if !strings.HasSuffix(path, ".iso") {
			log.Fatalln("Aborting:", path,
				"--cdrom payload MUST end in '.iso'.")
		}
		if _, err := os.Stat(path); err != nil {
			log.Fatalln("Aborting:", path, "provided as --cdrom payload",
				"not a valid file path.")
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			log.Fatalln("Aborting: couldn't get absolute path of", path)
		}
		vm.Storage.CDDrives = make(map[string]StorageDevice, 0)
		vm.Storage.CDDrives["0"] = StorageDevice{
			Type: CDROM,
			Slot: 0,
			Path: abs,
		}
	}
}

func (vm *VMInfo) validateNetworkInterfaces(nics []string) {
	for _, j := range nics {
		if len(j) > 0 {
			if strings.HasPrefix(j, "eth") {
				r, _ := regexp.Compile("eth([0-9]{1})$")
				if !r.MatchString(j) {
					log.Fatalln("Aborting: --net", j,
						"not in a reasonable format (eth|tap)[0-9]{1}. ")
				}
				slot, _ := strconv.Atoi(string(j[len(j)-1]))
				if vm.Network.Raw == nil {
					vm.Network.Raw = make(map[string]NetworkInterface, 0)
				}
				cd := vm.Network.Raw
				k := strconv.Itoa(slot)
				if _, ok := cd[k]; ok {
					log.Fatalln("Aborting: attempting to define",
						j, "twice")
				}
				kp := strconv.Itoa(slot - 1)
				_, ok := cd[kp]
				if !(slot == 0 || ok) {
					log.Fatalln("Aborting: cannot spec slot",
						fmt.Sprintf("'tap%d'", slot),
						"without slot",
						fmt.Sprintf("'tap%d'", slot-1),
						"populated in advance")
				}
				cd[k] = NetworkInterface{
					Type: Raw,
					Slot: slot,
				}
			} else if strings.HasPrefix(j, "tap") {
				r, _ := regexp.Compile("tap([0-9]{1})$")
				if !r.MatchString(j) {
					log.Fatalln("Aborting: --net", j,
						"not in a reasonable format (eth|tap)[0-9]{1}$. ")
				}
				log.Println("Tap interfaces not yet supported. ignoring")
			} else {
				log.Fatalln("Aborting: --net", j,
					"not in a reasonable format (eth|tap)[0-9]{1}$. ")
			}
		}
	}
}

func (vm *VMInfo) validateVolumes(volumes []string, root bool) {
	for _, j := range volumes {
		if j != "" {
			if !strings.HasSuffix(j, ".img") {
				log.Fatalln("Aborting:", j,
					"--volume payload MUST end in '.img'.")
			}
			if _, err := os.Stat(j); err != nil {
				log.Fatalln("Aborting:", j, "provided as --volume payload",
					"not a valid file path.")
			}
			abs, err := filepath.Abs(j)
			if err != nil {
				log.Fatalln("Aborting: couldn't get absolute path of", j)
			}
			if vm.Storage.HardDrives == nil {
				vm.Storage.HardDrives = make(map[string]StorageDevice, 0)
			}
			slot := len(vm.Storage.HardDrives)
			for _, z := range vm.Storage.HardDrives {
				if z.Path == abs {
					log.Fatalln("Aborting: attempting to set", j,
						"as base of multiple volumes.")
				}
			}
			vm.Storage.HardDrives[strconv.Itoa(slot)] = StorageDevice{
				Type: HDD,
				Slot: slot,
				Path: abs,
			}
			if root {
				vm.Root = strconv.Itoa(slot)
			}
		}
	}
}
