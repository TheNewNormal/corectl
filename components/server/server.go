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
	"os/signal"
	"sync"
	"syscall"

	"golang.org/x/net/context"

	"github.com/blang/semver"
	"github.com/braintree/manners"
	"github.com/coreos/etcd/client"
	"github.com/deis/pkg/log"
	"github.com/genevera/corectl/components/host/session"
	"github.com/genevera/corectl/release"
)

type (
	// MediaAssets ...
	MediaAssets map[string]semver.Versions

	// Config ...
	ServerContext struct {
		Meta              *release.Info
		Media             MediaAssets
		Active            VMmap
		APIserver         *manners.GracefulServer
		EtcdServer        *EtcdServer
		EtcdClient        client.KeysAPI
		DNSServer         *DNSServer
		Jobs              sync.WaitGroup
		AcceptingRequests bool
		WorkingNFS        bool
		Oops              chan error
		sync.Mutex
	}
)

var Daemon *ServerContext

// New ...
func New() (cfg *ServerContext) {
	return &ServerContext{
		Meta:              session.Caller.Meta,
		Jobs:              sync.WaitGroup{},
		AcceptingRequests: true,
		WorkingNFS:        false,
		Oops:              make(chan error, 1),
		Active:            make(VMmap),
	}
}

// Start server...
func Start() (err error) {
	// var  closeVPNhooks func()
	if !session.Caller.Privileged {
		return fmt.Errorf("not enough previleges to start server. " +
			"please use 'sudo'")
	}

	if err = Daemon.NewEtcd(EtcdClientURLs, EtcdPeerURLs,
		"corectld."+LocalDomainName, session.Caller.EtcDir()); err != nil {
		return
	}
	defer Daemon.EtcdServer.Stop()

	// we don't want skydns' data to persist at all across sessions
	Daemon.EtcdClient.Delete(context.Background(), "/skydns",
		&client.DeleteOptions{Dir: true, Recursive: true})

	if isPortOpen("tcp", ":"+EmbeddedDNSport) {
		return fmt.Errorf("Unable to start embedded skydns "+
			"as something else seems to be already binding hosts' port :%v",
			EmbeddedDNSport)
	}
	log.Info("starting embedded name server")
	if err = Daemon.NewDNSServer(LocalDomainName, ":"+EmbeddedDNSport,
		RecursiveNameServers); err != nil {
		return
	}
	defer Daemon.DNSServer.Stop()

	log.Info("checking nfs host settings")
	if err = nfsSetup(); err != nil {
		log.Warn("Unable to setup NFS. " +
			"No NFS facilities will be exposed to the VMs")
		log.Warn("%v", err)
	} else {
		Daemon.WorkingNFS = true
		log.Info("VMs will be able to have host's homedir shared via NFS")
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
		Daemon.Active.array().gracefullyShutdown()
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

	select {
	case err = <-Daemon.Oops:
		Daemon.Lock()
		Daemon.AcceptingRequests = false
		Daemon.Unlock()

		if err != nil {
			log.Err("OOPS %v", err.Error())
		}
	}

	Daemon.Jobs.Wait()
	Daemon.APIserver.Close()
	log.Info("gone!")
	return
}
