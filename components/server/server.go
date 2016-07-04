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

	"github.com/TheNewNormal/corectl/components/host/session"
	"github.com/TheNewNormal/corectl/release"
	"github.com/blang/semver"
	"github.com/braintree/manners"
	"github.com/helm/helm/log"
)

type (
	// MediaAssets ...
	MediaAssets map[string]semver.Versions

	// Config ...
	Config struct {
		sync.Mutex
		Meta      *release.Info
		Media     MediaAssets
		Active    map[string]*VMInfo
		APIserver *manners.GracefulServer
		Jobs      sync.WaitGroup
	}
)

var Daemon *Config

// New ...
func New() *Config {
	return &Config{
		Meta:      session.Caller.Meta,
		Media:     nil,
		Active:    nil,
		APIserver: nil,
		Jobs:      sync.WaitGroup{},
	}
}

// Start server...
func Start() (err error) {
	// var  closeVPNhooks func()
	if !session.Caller.Privileged {
		return fmt.Errorf("not enough previleges to start server. " +
			"please use 'sudo'")
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
