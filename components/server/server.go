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
		sync.Mutex
		DataStore client.KeysAPI
		Meta      *release.Info
		Media     MediaAssets
		Active    map[string]*VMInfo
		APIserver *manners.GracefulServer
		Jobs      sync.WaitGroup
	}
)

var Daemon *ServerContext

// New ...
func New() (cfg *ServerContext) {
	return &ServerContext{
		DataStore: nil,
		Meta:      session.Caller.Meta,
		Media:     nil,
		Active:    nil,
		APIserver: nil,
		Jobs:      sync.WaitGroup{},
	}
}

// Start server...
func Start() (err error) {
	var (
		hostname string
		etcdc    client.Client
	)
	// var  closeVPNhooks func()
	if !session.Caller.Privileged {
		return fmt.Errorf("not enough previleges to start server. " +
			"please use 'sudo'")
	}

	if hostname, err = os.Hostname(); err != nil {
		return
	}
	etcd := exec.Command(path.Join(session.ExecutableFolder(), "corectld.store"),
		"-data-dir="+session.Caller.EtcDir(),
		"-name="+hostname,
		"--listen-client-urls=http://0.0.0.0:2379,http://0.0.0.0:4001",
		"--advertise-client-urls=http://0.0.0.0:2379,http://0.0.0.0:4001")
	if log.IsDebugging {
		etcd.Stdout = os.Stdout
		etcd.Stderr = os.Stderr
	}
	etcd.Args[0] = "etcd"
	log.Info("starting embedded etcd")
	if err = etcd.Start(); err != nil {
		return
	}
	etcdS := client.Config{
		Endpoints: []string{"http://127.0.0.1:2379"},
		Transport: client.DefaultTransport,
		// set timeout per request to fail fast when the target endpoint is unavailable
		HeaderTimeoutPerRequest: 500 * time.Millisecond,
	}
	if etcdc, err = client.New(etcdS); err != nil {
		return
	}
	Daemon.DataStore = client.NewKeysAPI(etcdc)

	Daemon.DataStore.Delete(context.Background(),
		"/skydns", &client.DeleteOptions{Dir: true, Recursive: true})

	dnsArgs := []string{"-nameservers=8.8.8.8:53,8.8.4.4:53",
		"-domain=coreos.local",
		"-addr=0.0.0.0:53"}
	if log.IsDebugging {
		dnsArgs = append(dnsArgs, "-verbose")
	}
	skydns := exec.Command(path.
		Join(session.ExecutableFolder(), "corectld.nameserver"),
		dnsArgs...,
	)
	log.Info("starting embedded name server")
	if err = skydns.Start(); err != nil {
		return
	}

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
		os.Interrupt,
		syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		s := <-hades
		log.Info("Got '%v' signal, stopping server...", s)
		signal.Stop(hades)
		Daemon.Lock()
		Daemon.APIserver.Close()
		Daemon.Unlock()
	}()

	log.Info("server starting...")

	httpServiceSetup()
	rpcServiceSetup()

	Daemon.APIserver = manners.NewWithServer(&http.Server{
		Addr:    ":2511",
		Handler: httpServices})

	if err = Daemon.APIserver.ListenAndServe(); err != nil {
		return
	}

	Daemon.Lock()
	for _, r := range Daemon.Active {
		r.halt()
	}
	Daemon.Unlock()
	Daemon.Jobs.Wait()

	log.Info("gone!")
	return
}
