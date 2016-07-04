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
	"bytes"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"

	"os"
	"path"
	"time"

	"github.com/TheNewNormal/corectl/components/host/darwin/misc/uuid2ip"
	"github.com/TheNewNormal/corectl/components/host/session"
	"github.com/TheNewNormal/corectl/release"
	"github.com/blang/semver"
	"github.com/helm/helm/log"
	"github.com/gorilla/rpc"
	"github.com/gorilla/rpc/json"
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
		MAC            string
		UUID, original = args.Input[0], args.Input[1]
	)
	log.Debug("vm:uuid2mac")

	// handles UUIDs
	if _, found := Daemon.Active[UUID]; found {
		err = fmt.Errorf("Aborted: Another VM is "+
			"already running with the exact same UUID (%s)", UUID)
	} else {
		for {
			// we keep the loop just in case as the check
			// above is supposed to avoid most failures...
			// XXX
			if MAC, err =
				uuid2ip.GuestMACfromUUID(UUID); err == nil {
				// var ip string
				// if ip, err = uuid2ip.GuestIPfromMAC(MAC); err == nil {
				// 	log.Info("GUEST IP will be %v", ip)
				break
				// }
			}
			fmt.Println("=>", original, err)
			if original != "random" {
				log.Warn("unable to guess the MAC Address from the provided "+
					"UUID (%s). Using a randomly generated one\n", original)
			}
			UUID = uuid.NewV4().String()
		}
	}
	reply.Output = []string{MAC, strings.ToUpper(UUID)}
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
		err = fmt.Errorf("Couldn't decode response. %s", err)
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
