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
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"os"
	"path"
	"time"

	"github.com/TheNewNormal/corectl/components/host/session"
	"github.com/TheNewNormal/corectl/release"
	"github.com/blang/semver"
	"github.com/gorilla/rpc"
	"github.com/gorilla/rpc/json"
	"github.com/helm/helm/log"
	"github.com/satori/go.uuid"
)

var rpcServices = rpc.NewServer()

type (
	RPCservice struct{}

	RPCquery struct {
		Input []string
		VM    *VMInfo
	}
	RPCreply struct {
		Output  []string
		Meta    *release.Info
		VM      *VMInfo
		Images  map[string]semver.Versions
		Running map[string]*VMInfo
	}
)

func rpcServiceSetup() {
	rpcServices.RegisterCodec(json.NewCodec(), "application/json")
	rpcServices.RegisterCodec(json.NewCodec(), "application/json;charset=UTF-8")
	rpcServices.RegisterService(new(RPCservice), "")
	httpServices.Handle("/rpc", rpcServices)
}

func (s *RPCservice) Echo(r *http.Request,
	args *RPCquery, reply *RPCreply) (err error) {
	log.Debug("ping")
	reply.Meta = Daemon.Meta
	return
}

func (s *RPCservice) AvailableImages(r *http.Request,
	args *RPCquery, reply *RPCreply) (err error) {
	log.Debug("images:list")
	Daemon.Lock()
	defer Daemon.Unlock()
	reply.Images, err = localImages()
	return
}

func (s *RPCservice) RemoveImage(r *http.Request,
	args *RPCquery, reply *RPCreply) (err error) {
	var (
		channel, version = args.Input[0], args.Input[1]
		x                int
		y                semver.Version
	)

	log.Debug("images:remove")
	Daemon.Lock()

	for x, y = range Daemon.Media[channel] {
		if version == y.String() {
			break
		}
	}

	log.Debug("removing %v/%v", channel, version)

	Daemon.Media[channel] =
		append(Daemon.Media[channel][:x], Daemon.Media[channel][x+1:]...)
	Daemon.Unlock()

	log.Debug("%s/%s was made unavailable", channel, version)

	if err = os.RemoveAll(path.
		Join(session.Caller.ImageStore(), channel, y.String())); err != nil {
		log.Err(err.Error())
		return
	}

	reply.Images, err = localImages()
	return
}

func (s *RPCservice) UUIDtoMACaddr(r *http.Request,
	args *RPCquery, reply *RPCreply) (err error) {
	var (
		i              int
		macAddr        string
		stdout         io.ReadCloser
		UUID, original = args.Input[0], args.Input[1]
	)
	log.Debug("vm:uuid2mac (%v:%v)", args.Input[0], args.Input[1])

	// handles UUIDs
	if _, found := Daemon.Active[UUID]; found {
		err = fmt.Errorf("Aborted: Another VM is already running with the "+
			"exact same UUID [%s]", UUID)
	} else {
		for i < 3 {
			//
			// we just can't call uuid2ip.GuestMACfromUUID(UUID) directly here.
			//
			// according to https://developer.apple.com/library/mac/documentation/DriversKernelHardware/Reference/vmnet/
			// one "can create a maximum of 32 interfaces with a limit of
			// 4 per guest operating system" which in practice means that a
			// given Pid/corectld instance in aggregate can't create more than
			// 128 VMs (interfaces).
			// by doing the lookup as an external process that we "unrelate"
			// from its parent we get around this limitation and so each
			// corectld session stops having an 2ˆ7 upper bound on the number
			//  the VMs it can create over its lifetime
			//
			cmd := exec.Command(session.Executable(), "uuid2mac", UUID)
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Setpgid: true,
				Setsid:  false,
				Pgid:    0,
			}
			if stdout, err = cmd.StdoutPipe(); err != nil {
				break
			}
			rd := bufio.NewReader(stdout)
			if err = cmd.Start(); err != nil {
				break
			}
			macAddr, _ = rd.ReadString('\n')
			macAddr = strings.TrimSuffix(macAddr, "\n")
			if err = cmd.Wait(); err == nil {
				if len(macAddr) > 0 {
					if _, found := Daemon.Active[UUID]; !found {
						// unlikely statistically but ...
						break
					}
				}
			}
			log.Debug("=> %v:%v [%v]", original, err, i)
			if original != "random" {
				log.Warn("unable to guess the MAC Address from the provided "+
					"UUID (%s). Using a randomly generated one\n", original)
			}
			UUID = uuid.NewV4().String()
			i += 1
		}
		if i == 3 && err != nil {
			err = fmt.Errorf("Something went very wrong, as we're unable to " +
				"generate a MAC address from the provided UUID. Please fill " +
				"a bug at https://github.com/TheNewNormal/corectl/issues with " +
				"this error and wait there for our feedback...")
		}
	}
	reply.Output = []string{macAddr, strings.ToUpper(UUID)}
	return
}

func (s *RPCservice) Run(r *http.Request,
	args *RPCquery, reply *RPCreply) (err error) {
	log.Debug("vm:run")

	var bootArgs []string
	vm := args.VM

	vm.publicIPCh = make(chan string, 1)
	vm.errCh = make(chan error, 1)
	vm.done = make(chan struct{})

	if err = vm.register(); err != nil {
		return
	}

	if bootArgs, err = vm.assembleBootPayload(); err != nil {
		return
	}
	if err = vm.MkRunDir(); err != nil {
		return
	}
	vm.CreationTime = time.Now()

	payload := append(strings.Split(bootArgs[0], " "),
		"-f", fmt.Sprintf("%s%v", bootArgs[1], bootArgs[2]))
	vm.exec = exec.Command(filepath.Join(session.ExecutableFolder(),
		"corectld.runner"), payload...)

	go func() {
		timeout := time.After(ServerTimeout)
		select {
		case <-timeout:
			vm.Pid = vm.exec.Process.Pid
			vm.halt()
			vm.errCh <- fmt.Errorf("Unable to grab VM's IP after " +
				"30s (!)... Aborted")
		case ip := <-vm.publicIPCh:
			Daemon.Lock()
			vm.Pid, vm.PublicIP = vm.exec.Process.Pid, ip
			Daemon.Unlock()
			close(vm.publicIPCh)
			close(vm.done)
			log.Info("started '%s' in background with IP %v and "+
				"PID %v\n", vm.Name, vm.PublicIP, vm.exec.Process.Pid)
		}
	}()

	go func() {
		Daemon.Jobs.Add(1)
		defer Daemon.Jobs.Done()
		Daemon.Lock()
		err := vm.exec.Start()
		Daemon.Unlock()
		if err != nil {
			vm.errCh <- err
		}
		vm.exec.Wait()
		vm.deregister()
		os.Remove(vm.TTY())
		// give it time to flush logs
		time.Sleep(3 * time.Second)
	}()

	select {
	case <-vm.done:
		if len(vm.PublicIP) == 0 {
			err = fmt.Errorf("VM terminated abnormally too early")
		}
		reply.VM = vm
		return
	case err = <-vm.errCh:
		return
	}

	return
}

func (s *RPCservice) Stop(r *http.Request,
	args *RPCquery, reply *RPCreply) (err error) {
	log.Debug("server:stop")
	log.Info("Sky must be falling. Shutting down...")

	Daemon.APIserver.Close()
	return
}
func (s *RPCservice) ActiveVMs(r *http.Request,
	args *RPCquery, reply *RPCreply) (err error) {
	log.Debug("vm:list")
	reply.Running = Daemon.Active
	return
}

func (s *RPCservice) StopVMs(r *http.Request,
	args *RPCquery, reply *RPCreply) (err error) {
	log.Debug("vm:stop")

	var toHalt []string

	if len(args.Input) == 0 {
		for _, x := range Daemon.Active {
			toHalt = append(toHalt, x.UUID)
		}
	} else {
		for _, t := range args.Input {
			for _, v := range Daemon.Active {
				if v.Name == t || v.UUID == t {
					toHalt = append(toHalt, v.UUID)
				}
			}
		}
	}
	for _, v := range toHalt {
		Daemon.Active[v].halt()
		for {
			Daemon.Lock()
			_, stillAlive := Daemon.Active[v]
			Daemon.Unlock()
			if !stillAlive {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	return
}

func RPCQuery(f string, args *RPCquery) (reply *RPCreply, err error) {
	var (
		message []byte
		req     *http.Request
		resp    *http.Response
		client  = new(http.Client)
		server  = "http://" + session.Caller.ServerAddress + "/rpc"
	)
	if message, err =
		json.EncodeClientRequest("RPCservice."+f, args); err != nil {
		return
	}
	if req, err =
		http.NewRequest("POST", server, bytes.NewBuffer(message)); err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if resp, err = client.Do(req); err != nil {
		err = fmt.Errorf("Error in sending request to %s. %s", server, err)
		return
	}
	defer resp.Body.Close()
	if err = json.DecodeClientResponse(resp.Body, &reply); err != nil {
		err = fmt.Errorf("%s", err)
	}
	return
}

func (cfg *Config) Running() (i *release.Info, err error) {
	reply := &RPCreply{}
	if reply, err = RPCQuery("Echo", &RPCquery{}); err != nil {
		err = session.ErrServerUnreachable
	} else {
		i = reply.Meta
	}
	return
}
