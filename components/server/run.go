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

package server

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os/exec"
	"sort"
	"syscall"

	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/genevera/corectl/components/host/session"
	"github.com/deis/pkg/log"
	"github.com/dustin/go-humanize"
	"golang.org/x/crypto/ssh"
)

type (
	// VMInfo - per VM settings
	VMInfo struct {
		Name, Channel, Version, UUID            string
		MacAddress, PublicIP                    string
		InternalSSHkey, InternalSSHprivate      string
		Cpus, Memory, Pid                       int
		SSHkey, CloudConfig, CClocation         string `json:",omitempty"`
		AddToHypervisor, AddToKernel            string `json:",omitempty"`
		Ethernet                                []NetworkInterface
		Storage                                 StorageAssets `json:",omitempty"`
		SharedHomedir, OfflineMode, NotIsolated bool
		FormatRoot, PersistentRoot              bool
		CreationTime                            time.Time

		publicIPCh               chan string
		errCh                    chan error
		done                     chan struct{}
		exec                     *exec.Cmd
		isolationCheck, callBack sync.Once
		cloudConfigContents      []byte
	}
	//
	VMmap map[string]*VMInfo
	// Config ...
	VMs []*VMInfo
	// NetworkInterface ...
	NetworkInterface struct {
		Type int
		// if/when tap...
		Path string `json:",omitempty"`
	}
	// StorageDevice ...
	StorageDevice struct {
		Slot, Format int
		Type, Path   string
	}
	// StorageAssets ...
	StorageAssets struct {
		CDDrives, HardDrives map[string]StorageDevice `json:",omitempty"`
	}
)

const (
	_ = iota
	Raw
	Qcow2
	Tap
	HDD    = "HDD"
	CDROM  = "CDROM"
	Local  = "localfs"
	Remote = "URL"
)

var ServerTimeout = 25 * time.Second

// ValidateCDROM ...
func (vm *VMInfo) ValidateCDROM(path string) (err error) {
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

// ValidateVolumes ...
func (vm *VMInfo) ValidateVolumes(volumes []string, root bool) (err error) {
	var (
		abs    string
		fh     *os.File
		format = Qcow2
	)

	for _, j := range volumes {
		if j != "" {
			if _, err = os.Stat(j); err != nil {
				return
			}
			if abs, err = filepath.Abs(j); err != nil {
				return
			}
			if fh, err = os.Open(j); err != nil {
				return
			}
			defer fh.Close()
			if err = ValidateQcow2(fh); err != nil {
				if err != ErrFileIsNotQCOW2 {
					return
				}
				log.Warn("using Raw formated volumes is a deprecated feature " +
					"that may become unsupported in the future. Please " +
					"consider moving to QCOW2 ones")
				format = Raw
				err = nil
			}
			if format == Raw {
				// to be consistent with previous behaviour
				if !strings.HasSuffix(j, ".img") {
					return fmt.Errorf("Aborting: --volume payload MUST end"+
						" in '.img' ('%s' doesn't)", j)
				}
			}
			// check atomicity
			reply := &RPCreply{}

			if reply, err = RPCQuery("ActiveVMs", &RPCquery{}); err != nil {
				return
			}

			for _, d := range reply.Running {
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
			vm.Storage.HardDrives[strconv.Itoa(slot)] =
				StorageDevice{Type: HDD, Format: format, Slot: slot, Path: abs}
			if root {
				vm.PersistentRoot = root
			}
		}
	}
	return
}

// ValidateCloudConfig ...
func (vm *VMInfo) ValidateCloudConfig(config string) (err error) {
	var response *http.Response

	if len(config) > 0 {
		if response, err = http.Get(config); response != nil {
			response.Body.Close()
		}

		vm.CloudConfig = config

		if err == nil && (response.StatusCode == http.StatusOK ||
			response.StatusCode == http.StatusNoContent) {
			vm.CClocation = Remote
			return
		}

		if vm.CloudConfig, err = filepath.Abs(config); err != nil {
			return
		}
		if _, err = os.Stat(vm.CloudConfig); err != nil {
			return
		}
		vm.CClocation = Local
	}
	return
}

// SSHkeyGen creates a one-time ssh public and private key pair
func (vm *VMInfo) SSHkeyGen() (err error) {
	var (
		public ssh.PublicKey
		secret *rsa.PrivateKey
	)

	if secret, err = rsa.GenerateKey(rand.Reader, 2014); err != nil {
		return
	}

	secretDer := x509.MarshalPKCS1PrivateKey(secret)
	secretBlk := pem.Block{
		Type: "RSA PRIVATE KEY", Headers: nil, Bytes: secretDer,
	}
	if public, err = ssh.NewPublicKey(&secret.PublicKey); err != nil {
		return
	}

	vm.InternalSSHprivate = string(pem.EncodeToMemory(&secretBlk))
	vm.InternalSSHkey =
		strings.TrimSuffix(string(ssh.MarshalAuthorizedKey(public)), "\n")
	return
}

func (vm *VMInfo) assembleBootPayload() (xArgs []string, err error) {
	var (
		cmdline = "earlyprintk=serial console=ttyS0 coreos.autologin coreos.first_boot=1"
		prefix  = "coreos_production_pxe"
		vmlinuz = fmt.Sprintf("%s/%s/%s/%s.vmlinuz",
			session.Caller.ImageStore(), vm.Channel, vm.Version,
			prefix)
		initrd = fmt.Sprintf("%s/%s/%s/%s_image.cpio.gz",
			session.Caller.ImageStore(), vm.Channel, vm.Version,
			prefix)
		instr = []string{
			"-s", "0:0,hostbridge",
			"-l", "com1,autopty=" + vm.TTY() + ",log=" + vm.Log(),
			"-s", "5,virtio-rnd",
			"-s", "31,lpc",
			"-U", vm.UUID,
			"-m", fmt.Sprintf("%vM", vm.Memory),
			"-c", fmt.Sprintf("%v", vm.Cpus),
			"-A",
			"-u",
		}
	)
	if vm.PersistentRoot {
		cmdline = fmt.Sprintf("%s root=LABEL=ROOT", cmdline)
	}

	cmdline = fmt.Sprintf("%s corectl.hostname=%s  coreos.config.url=%s",
		cmdline, vm.Name, vm.endpoint()+"/ignition")

	if vm.CloudConfig != "" {
		if vm.CClocation == Local {
			cmdline = fmt.Sprintf("%s cloud-config-url=%s",
				cmdline, vm.endpoint()+"/cloud-config")
		} else {
			cmdline = fmt.Sprintf("%s cloud-config-url=%s",
				cmdline, vm.CloudConfig)
		}
	}

	if vm.AddToHypervisor != "" {
		instr = append(instr, vm.AddToHypervisor)
	}

	if vm.AddToKernel != "" {
		cmdline = fmt.Sprintf("%s %s", cmdline, vm.AddToKernel)
	}

	for v, vv := range vm.Ethernet {
		if vv.Type == Tap {
			instr = append(instr, "-s",
				fmt.Sprintf("2:%d,virtio-tap,%v", v, vv.Path))
		} else {
			instr = append(instr, "-s",
				fmt.Sprintf("2:%d,virtio-net", v))
		}
	}

	for _, v := range vm.Storage.CDDrives {
		instr = append(instr, "-s", fmt.Sprintf("3:%d,ahci-cd,%s",
			v.Slot, v.Path))
	}

	for _, v := range vm.Storage.HardDrives {
		switch v.Format {
		case Raw:
			instr = append(instr, "-s", fmt.Sprintf("4:%d,virtio-blk,%s",
				v.Slot, v.Path))
		case Qcow2:
			instr = append(instr, "-s",
				fmt.Sprintf("4:%d,virtio-blk,file://%s,format=qcow",
					v.Slot, v.Path))
		}
	}

	return []string{strings.Join(instr, " "),
			fmt.Sprintf("kexec,%s,%s,", vmlinuz, initrd),
			fmt.Sprintf("%v", cmdline)},
		err
}

func (list VMs) gracefullyShutdown() {
	sort.Sort(sort.Reverse(VMs(list)))

	for _, v := range list {
		log.Info("shutting down %v...", v.Name)
		Daemon.Active[v.UUID].gracefullyShutdown()
		for {
			Daemon.Lock()
			_, stillAlive := Daemon.Active[v.UUID]
			Daemon.Unlock()
			if !stillAlive {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	return
}

func (vm *VMInfo) kill() {
	log.Debug("hard killing %v", vm.Name)
	if err := vm.exec.Process.Kill(); err != nil {
		log.Err(err.Error())
	}
}

func (vm *VMInfo) gracefullyShutdown() {
	// Try to gracefully terminate the process.
	if err := vm.exec.Process.Signal(syscall.SIGTERM); err != nil {
		log.Err(err.Error())
	}
	select {
	case <-time.After(ServerTimeout):
		log.Err("Attempting to halt %v (%v) with SIGTERM timed out, "+
			"SIGKILLing it now.", vm.UUID, vm.exec.Process.Pid)
		vm.kill()
	case <-vm.done:
	}
}

func (vm *VMInfo) lookup() bool {
	Daemon.Lock()
	defer Daemon.Unlock()
	// handles UUIDs
	if _, ok := Daemon.Active[vm.UUID]; ok {
		return true
	}
	for _, v := range Daemon.Active {
		if v.Name == vm.Name {
			return true
		}
	}
	return false
}

func (vm *VMInfo) register() (err error) {
	if vm.Name == "corectld" {
		return fmt.Errorf("attempting to name a VM with the (only) " +
			"reserved hostname ")
	}

	str := fmt.Sprintf("'%v'", vm.Name)
	if vm.Name != vm.UUID {
		str = fmt.Sprintf("'%v' (%v)", vm.Name, vm.UUID)
	}

	if vm.lookup() {
		err = fmt.Errorf("Aborted: Another VM is "+
			"already running with the same name or UUID (%s)", str)
	} else {
		Daemon.Active[vm.UUID] = vm
		log.Info("registered %s", str)

	}
	return
}

func (vm *VMInfo) deregister() {
	str := fmt.Sprintf("'%v'", vm.Name)
	if vm.Name != vm.UUID {
		str = fmt.Sprintf("'%v' (%v)", vm.Name, vm.UUID)
	}
	Daemon.DNSServer.rmRecord(vm.Name, vm.PublicIP)
	log.Info("unregistered %s as it's gone", str)
	delete(Daemon.Active, vm.UUID)

}

func (vm *VMInfo) endpoint() string {
	return fmt.Sprintf("http://%s:%s/%s",
		session.Caller.Address, "2511", vm.UUID)
}

func (vm *VMInfo) RunDir() string {
	return filepath.Join(session.Caller.RunDir(), vm.UUID)
}

func (vm *VMInfo) MkRunDir() error {
	rundir := vm.RunDir()
	if _, e := os.Stat(rundir); e == nil {
		log.Warn("%v already exists - reusing it.", rundir)
		return nil
	}
	log.Info("creating %v", rundir)
	return os.MkdirAll(rundir, 0755)
}

func (vm *VMInfo) Log() string {
	return filepath.Join(vm.RunDir(), "log")
}

func (vm *VMInfo) TTY() string {
	return filepath.Join(vm.RunDir(), "tty")
}

func (vm *VMInfo) PrettyPrint() {
	fmt.Printf("\n UUID:\t\t%v\n  Name:\t\t%v\n  Version:\t%v\n  "+
		"Channel:\t%v\n  vCPUs:\t%v\n  Memory (MB):\t%v\n",
		vm.UUID, vm.Name, vm.Version, vm.Channel, vm.Cpus, vm.Memory)
	fmt.Printf("  Pid:\t\t%v\n  Uptime:\t%v\n",
		vm.Pid, humanize.Time(vm.CreationTime))
	fmt.Printf("  Sees World:\t%v\n", vm.NotIsolated)
	if vm.CloudConfig != "" {
		fmt.Printf("  cloud-config:\t%v\n", vm.CloudConfig)
	}
	fmt.Println("  Network:")
	fmt.Printf("    eth0:\t%v\n", vm.PublicIP)
	vm.Storage.PrettyPrint(vm.PersistentRoot)
}

func (volumes *StorageAssets) PrettyPrint(root bool) {
	if len(volumes.CDDrives)+len(volumes.HardDrives) > 0 {
		fmt.Println("  Volumes:")
		for a, b := range volumes.CDDrives {
			fmt.Printf("   /dev/cdrom%v\t%s\n", a, b.Path)
		}
		for a, b := range volumes.HardDrives {
			format := "raw"
			i, _ := strconv.Atoi(a)
			if b.Format == Qcow2 {
				format = "qcow2"
			}
			if root && i == 0 {
				fmt.Printf("   /,/dev/vd%v\t%s,format=%s\n", string(i+'a'),
					b.Path, format)
			} else {
				fmt.Printf("   /dev/vd%v\t%s,format=%s\n", string(i+'a'),
					b.Path, format)
			}
		}
	}
}

func (run VMs) Len() int {
	return len(run)
}
func (run VMs) Swap(i, j int) {
	run[i], run[j] = run[j], run[i]
}
func (run VMs) Less(i, j int) bool {
	return run[i].CreationTime.Before(run[j].CreationTime)
}

func (in VMmap) array() (out VMs) {
	for _, r := range in {
		out = append(out, r)
	}
	return
}
