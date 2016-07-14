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
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/context"

	"github.com/TheNewNormal/corectl/components/host/session"
	"github.com/TheNewNormal/corectl/components/target/coreos"
	"github.com/TheNewNormal/corectl/release"
	"github.com/blang/semver"
	"github.com/braintree/manners"
	"github.com/coreos/etcd/client"
	"github.com/helm/helm/log"
)

type (
	// MediaAssets ...
	MediaAssets map[string]semver.Versions

	// Config ...
	ServerContext struct {
		DataStore         client.KeysAPI
		Meta              *release.Info
		Media             MediaAssets
		Active            map[string]*VMInfo
		APIserver         *manners.GracefulServer
		Jobs              sync.WaitGroup
		AcceptingRequests bool
		Oops              chan error
		sync.Mutex
	}
)

var Daemon *ServerContext

// New ...
func New() (cfg *ServerContext) {
	return &ServerContext{
		DataStore:         nil,
		Meta:              session.Caller.Meta,
		Media:             nil,
		Active:            nil,
		APIserver:         nil,
		Jobs:              sync.WaitGroup{},
		AcceptingRequests: true,
		Oops:              make(chan error, 1),
	}
}

// Start server...
func Start() (err error) {
	var (
		etcd, skydns *exec.Cmd
		etcdc        client.Client
	)
	// var  closeVPNhooks func()
	if !session.Caller.Privileged {
		return fmt.Errorf("not enough previleges to start server. " +
			"please use 'sudo'")
	}

	etcd = exec.Command(path.Join(session.ExecutableFolder(), "corectld.store"),
		"-data-dir="+session.Caller.EtcDir(),
		"-name=corectld."+coreos.LocalDomainName,
		"--listen-client-urls=http://0.0.0.0:2379,http://0.0.0.0:4001",
		"--advertise-client-urls=http://0.0.0.0:2379,http://0.0.0.0:4001")
	if log.IsDebugging {
		etcd.Stdout = os.Stdout
		etcd.Stderr = os.Stderr
	}

	etcdS := client.Config{
		Endpoints:               []string{"http://127.0.0.1:2379"},
		Transport:               client.DefaultTransport,
		HeaderTimeoutPerRequest: 1 * time.Second,
	}
	if etcdc, err = client.New(etcdS); err != nil {
		return
	}
	Daemon.DataStore = client.NewKeysAPI(etcdc)

	if isPortOpen("tcp", "127.0.0.1:2379") {
		return fmt.Errorf("Unable to start embedded etcd, as another one " +
			"seems to be already running")
	}
	log.Info("starting embedded etcd")
	if err = etcd.Start(); err != nil {
		log.Err(err.Error())
		return
	}

	defer func() {
		etcd.Process.Signal(syscall.SIGTERM)
	}()
	select {
	case <-time.After(3 * time.Second):
		return fmt.Errorf("Unable to establish a meaningfull " +
			"conversation with our embedded etcd after 3 seconds...")
	case <-time.Tick(250 * time.Millisecond):
		if !isPortOpen("tcp", "127.0.0.1:2379") {
			break
		}
	}
	log.Info("embedded etcd is ready")
	// we don't want skydns' data to persist at all across sessions
	Daemon.DataStore.Delete(context.Background(), "/skydns",
		&client.DeleteOptions{Dir: true, Recursive: true})

	dnsArgs := []string{"-nameservers=8.8.8.8:53,8.8.4.4:53",
		"-domain=" + coreos.LocalDomainName,
		"-addr=0.0.0.0:53"}
	if log.IsDebugging {
		dnsArgs = append(dnsArgs, "-verbose")
	}
	skydns = exec.Command(path.
		Join(session.ExecutableFolder(), "corectld.nameserver"),
		dnsArgs...,
	)
	log.Info("starting embedded name server")
	if err = skydns.Start(); err != nil {
		return
	}
	// register our host
	if err = skyWrite("corectld", session.Caller.Network.Address); err != nil {
		return
	}
	defer func() {
		skyWipe("corectld", session.Caller.Network.Address)
		skydns.Process.Signal(syscall.SIGTERM)
	}()

	log.Info("checking nfs host settings")
	if err = nfsSetup(); err != nil {
		return
	}
	// log.Info("checking for VPN setups")
	// if closeVPNhooks, err = HandleVPNtunnels(); err != nil {
	// 	return
	// }
	// defer closeVPNhooks()

	log.Info("registering locally available images")
	if Daemon.Media, err = localImages(); err != nil {
		return
	}
	hades := make(chan os.Signal, 1)
	signal.Notify(hades,
		syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		s := <-hades
		log.Info("Got '%v' signal, stopping server...", s)
		signal.Stop(hades)
		Daemon.Oops <- nil
	}()

	log.Info("server starting...")

	httpServiceSetup()
	rpcServiceSetup()

	go func() {
		Daemon.Lock()
		Daemon.APIserver = manners.NewWithServer(&http.Server{
			Addr:    ":2511",
			Handler: httpServices})
		Daemon.Unlock()
		if err := Daemon.APIserver.ListenAndServe(); err != nil {
			Daemon.Oops <- err
		}
		Daemon.Oops <- nil
	}()

	go func() {
		etcd.Wait()
		Daemon.Lock()
		v := Daemon.AcceptingRequests
		Daemon.Unlock()
		if v {
			// ended too early
			Daemon.Oops <- fmt.Errorf("corectld.store (etcd) ended " +
				"abnormaly. Shutting down")
		}
	}()
	go func() {
		skydns.Wait()
		Daemon.Lock()
		v := Daemon.AcceptingRequests
		Daemon.Unlock()
		if v {
			// ended too early
			Daemon.Oops <- fmt.Errorf("corectld.nameserver (skydns) " +
				"ended abnormaly. Shutting down")
		}
	}()
	select {
	case err = <-Daemon.Oops:
		Daemon.Lock()
		Daemon.AcceptingRequests = false
		Daemon.Unlock()

		if err != nil {
			log.Err("OOPS %v", err.Error())
		}
	}

	Daemon.Lock()
	for _, r := range Daemon.Active {
		r.halt()
	}
	Daemon.Unlock()
	Daemon.Jobs.Wait()
	Daemon.APIserver.Close()
	log.Info("gone!")
	return
}
