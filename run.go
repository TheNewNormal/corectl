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

	"github.com/codegangsta/cli"
	"github.com/satori/go.uuid"
)

func runAction(c *cli.Context) {
	SessionContext.canRun()
	vm := &SessionContext.data

	vm.setChannel(c.String("channel"))
	vm.setVersion(c.String("version"))

	vm.lookupImage()

	vm.xhyveCheck(c.String("xhyve"))
	vm.tweakXhyve(c.String("extra"))

	vm.uuidCheck(c.String("uuid"))
	vm.validateCPU(c.String("cpus"))
	vm.validateRAM(c.String("memory"))
	vm.setSSHKey(c.String("sshkey"))

	vm.validateNetworkInterfaces(c.StringSlice("net"))
	vm.validateVolumes(c.StringSlice("volume"))
	vm.validateCloudConfig(c.String("cloud_config"))

	username, _ := user.LookupId(SessionContext.uid)
	cmdline := fmt.Sprintf("%s %s %s %s %s", "earlyprintk=serial",
		"console=ttyS0", "coreos.autologin",
		"localuser="+username.Username, "uuid="+vm.UUID)
	if vm.SSHkey != "" {
		cmdline = fmt.Sprintf("%s sshkey=\"%s\"", cmdline, vm.SSHkey)
	}
	vmlinuz := fmt.Sprintf("%s/images/%s/%s/coreos_production_pxe.vmlinuz",
		SessionContext.configDir, vm.Channel, vm.Version)
	initrd := fmt.Sprintf("%s/images/%s/%s/coreos_production_pxe_image.cpio.gz",
		SessionContext.configDir, vm.Channel, vm.Version)

	args := []string{
		"-s", "0:0,hostbridge",
		"-l", "com1,stdio",
		"-s", "31,lpc",
		"-U", vm.UUID,
		"-m", fmt.Sprintf("%sM", vm.Memory),
		"-c", vm.Cpus,
		"-A",
	}
	if vm.Extra != "" {
		args = append(args, vm.Extra)
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
		args = append(args, "-s", fmt.Sprintf("2:%d,virtio-net", v.Slot))
	}
	// for _, v := range vm.Network.Tap {
	// 	args = append(args, "-s", fmt.Sprintf("2:%d,virtio-tap,%s", v.Slot))
	// }

	for _, v := range vm.Storage.CDDrives {
		args = append(args, "-s", fmt.Sprintf("3:%d,ahci-cd,%s", v.Slot, v.Path))
	}
	for _, v := range vm.Storage.HardDrives {
		args = append(args, "-s", fmt.Sprintf("4:%d,virtio-blk,%s", v.Slot, v.Path))
	}

	usersDir := etcExports{}
	usersDir.share()

	cfg, _ := json.MarshalIndent(vm, "", "    ")
	fmt.Println(string(cfg))
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
	cmd := exec.Command(vm.Xhyve, append(args, "-f",
		fmt.Sprintf("kexec,%s,%s,%s", vmlinuz, initrd, cmdline))...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Println("xhyve exited with", err)
	}
	usersDir.unshare()
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
	}
	vm.Memory = ram
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
	// XXX we need to wipe -s x:y,... if passed thru here
	vm.Extra = extra
}

func (vm *VMInfo) validateNetworkInterfaces(nics []string) {
	if len(nics) > 0 {
		for _, j := range nics {
			if strings.HasPrefix(j, "eth") {
				r, _ := regexp.Compile("eth([0-9]{1})$")
				if !r.MatchString(j) {
					log.Fatalln("Aborting: --net", j,
						"not in a reasonable format (eth|tap)[0-9]{1}$,PATH. ")
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
						"not in a reasonable format (eth|tap)[0-9]{1}$,PATH. ")
				}
				log.Println("Tap interfaces not yet supported. ignoring")
			} else {
				log.Fatalln("Aborting: --net", j,
					"not in a reasonable format (eth|tap)[0-9]{1}$,PATH. ")
			}
		}
	}
}
func (vm *VMInfo) validateVolumes(volumes []string) {
	if len(volumes) > 0 {
		for _, j := range volumes {
			arr := strings.Split(j, ",")
			if len(arr) != 2 {
				log.Fatalln("Aborting: --volume", j,
					"not in a reasonable format (cdrom[0-9]|vd[a-z]),PATH. ")
			}
			if _, err := os.Stat(arr[1]); err != nil {
				log.Fatalln("Aborting:", arr[1], "not a valid file path")
			}
			if strings.HasPrefix(arr[0], "vd") {
				r, _ := regexp.Compile("vd([a-z]{1})$")
				if !r.MatchString(arr[0]) {
					log.Fatalln("Aborting: --volume", j,
						"not in a recognizable format",
						"- ((cdrom([0-9]{1})|vd([a-z]{1}))$,PATH")
				}
				slot := int(arr[0][2] - 'a')
				if vm.Storage.HardDrives == nil {
					vm.Storage.HardDrives = make(map[string]StorageDevice, 0)
				}
				hdd := vm.Storage.HardDrives
				k := strconv.Itoa(slot)
				if _, ok := hdd[k]; ok {
					log.Fatalln("Aborting: attempting to define",
						arr[0], "twice")
				}
				kp := strconv.Itoa(slot - 1)
				_, ok := hdd[kp]
				if !(slot == 0 || ok) {
					log.Fatalln("Aborting: cannot spec slot",
						fmt.Sprintf("'vd%s'", string('a'+slot)),
						"without slot",
						fmt.Sprintf("'vd%s'", string('a'+slot-1)),
						"populated in advance")
				}
				hdd[k] = StorageDevice{
					Type: HDD,
					Slot: slot,
					Path: filepath.Join(SessionContext.pwd,
						arr[1]),
				}

			} else if strings.HasPrefix(arr[0], "cdrom") {
				r, _ := regexp.Compile("cdrom([0-9]{1})$")
				if !r.MatchString(arr[0]) {
					log.Fatalln("Aborting: --volume", j,
						"not in a recognizable format",
						"- ((cdrom([0-9]{1})|vd([a-z]{1}))$,PATH")
				}
				slot, _ := strconv.Atoi(string(arr[0][len(arr[0])-1]))
				if vm.Storage.CDDrives == nil {
					vm.Storage.CDDrives = make(map[string]StorageDevice, 0)
				}
				cd := vm.Storage.CDDrives
				k := strconv.Itoa(slot)
				if _, ok := cd[k]; ok {
					log.Fatalln("Aborting: attempting to define",
						arr[0], "twice")
				}
				kp := strconv.Itoa(slot - 1)
				_, ok := cd[kp]
				if !(slot == 0 || ok) {
					log.Fatalln("Aborting: cannot spec slot",
						fmt.Sprintf("'cdrom%d'", slot),
						"without slot",
						fmt.Sprintf("'cdrom%d'", slot-1),
						"populated in advance")
				}
				cd[k] = StorageDevice{
					Type: CDROM,
					Slot: slot,
					Path: filepath.Join(SessionContext.pwd,
						arr[1]),
				}
			} else {
				log.Fatalln("Aborting: --volume", j,
					"not in a recognizable format",
					"- ((cdrom([0-9]{1})|vd([a-z]{1}))$,PATH")
			}
		}

	}
}

func runFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{
			Name:   "uuid",
			Value:  "random",
			Usage:  "VM's UUID",
			EnvVar: "COREOS_UUID,UUID",
		},
		cli.StringFlag{
			Name:   "memory",
			Value:  "1024",
			Usage:  "VM's memory",
			EnvVar: "COREOS_MEMORY,MEMORY,MEM",
		},
		cli.StringFlag{
			Name:   "cpus",
			Value:  "1",
			Usage:  "VM's CPUs #",
			EnvVar: "COREOS_CPUS,CPUS",
		},
		cli.StringFlag{
			Name:   "cloud_config,cloud-config",
			Usage:  "cloud-config file location (either URL or local path)",
			EnvVar: "COREOS_CLOUD_CONFIG,CLOUD_CONFIG",
		}, cli.StringFlag{
			Name:   "xhyve",
			Value:  "/usr/local/bin/xhyve",
			Usage:  "xhyve binary to use",
			EnvVar: "COREOS_XHYVE,XHYVE",
			// }, cli.StringFlag{
			//	Name:   "config,f",
			//	Usage:  "load VM configuration from file",
			//	EnvVar: "COREOS_CONFIG,CONFIG",
		}, cli.StringFlag{
			Name:   "sshkey",
			Value:  "",
			Usage:  "VM's default ssh key",
			EnvVar: "COREOS_SSHKEY,SSHKEY",
		}, cli.StringFlag{
			Name:   "extra",
			Value:  "",
			Usage:  "additional arguments to xhyve hypervisor",
			EnvVar: "COREOS_XHYVE_EXTRA,EXTRA_ARGS",
		}, cli.StringSliceFlag{
			Name:  "net",
			Usage: "append additional network interfaces to VM",
		}, cli.StringSliceFlag{
			Name:  "volume",
			Usage: "append disk volumes to VM",
		},
	}
}
