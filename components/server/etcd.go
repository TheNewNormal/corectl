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

//  adapted from github.com/kubernetes/minikube/pkg/localkube/etcd.go
//

package server

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/api/v2http"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/coreos/etcd/pkg/types"
)

const (
	EtcdName = "corectld.store"
)

var (
	// EtcdClientURLs have listeners created and handle etcd API traffic
	EtcdClientURLs = []string{"http://localhost:2379"}

	// EtcdPeerURLs don't have listeners created for them, they are used to pass
	// Etcd validation
	EtcdPeerURLs = []string{"http://0.0.0.0:2380"}
)

// Etcd is a Server which manages an Etcd cluster
type EtcdServer struct {
	*etcdserver.EtcdServer
	config        *etcdserver.ServerConfig
	clientListens []net.Listener
}

// NewEtcd creates a new default etcd Server using 'dataDir' for persistence.
// Panics if could not be configured.
func (d *ServerContext) NewEtcd(clientURLStrs,
	peerURLStrs []string, name, dataDirectory string) error {
	clientURLs, err := types.NewURLs(clientURLStrs)
	if err != nil {
		return err
	}

	peerURLs, err := types.NewURLs(peerURLStrs)
	if err != nil {
		return err
	}

	d.EtcdServer = &EtcdServer{
		config: &etcdserver.ServerConfig{
			Name:       name,
			ClientURLs: clientURLs,
			PeerURLs:   peerURLs,
			DataDir:    dataDirectory,
			InitialPeerURLsMap: map[string]types.URLs{
				name: peerURLs,
			},

			NewCluster: true,

			SnapCount:     etcdserver.DefaultSnapCount,
			MaxSnapFiles:  5,
			MaxWALFiles:   5,
			TickMs:        100,
			ElectionTicks: 10,
		},
	}
	if err := d.EtcdServer.Start(); err != nil {
		return err
	}

	// set client
	etcdc, err := client.
		New(client.Config{
			Endpoints:               clientURLStrs,
			Transport:               client.DefaultTransport,
			HeaderTimeoutPerRequest: 1 * time.Second,
		})
	if err != nil {
		return err
	}
	d.EtcdClient = client.NewKeysAPI(etcdc)
	return nil
}

// Start starts starts the etcd server and listening for client connections
func (e *EtcdServer) Start() (err error) {
	e.EtcdServer, err = etcdserver.NewServer(e.config)
	if err != nil {
		return fmt.Errorf("Etcd config error: %v", err)
	}

	// create client listeners
	clientListeners, err := createListeners(e.config.ClientURLs)
	if err != nil {
		return
	}
	// start etcd
	e.EtcdServer.Start()

	// setup client listeners
	ch := v2http.NewClientHandler(e.EtcdServer, e.requestTimeout())
	for _, l := range clientListeners {
		go func(l net.Listener) {
			srv := &http.Server{
				Handler:     ch,
				ReadTimeout: 5 * time.Minute,
			}
			panic(srv.Serve(l))
		}(l)
	}
	return
}

// Stop closes all connections and stops the Etcd server
func (e *EtcdServer) Stop() {
	if e.EtcdServer != nil {
		e.EtcdServer.Stop()
	}

	for _, l := range e.clientListens {
		l.Close()
	}
}

func (e *EtcdServer) requestTimeout() time.Duration {
	// from github.com/coreos/etcd/etcdserver/config.go
	return 5*time.Second + 2*time.Duration(e.config.ElectionTicks)*
		time.Duration(e.config.TickMs)*time.Millisecond
}

func createListeners(urls types.URLs) (listeners []net.Listener, err error) {
	for _, url := range urls {
		var l net.Listener
		if l, err = net.Listen("tcp", url.Host); err != nil {
			return
		}
		if l, err = transport.
			NewKeepAliveListener(l, url.Scheme, &tls.Config{}); err != nil {
			return
		}
		listeners = append(listeners, l)
	}
	return
}
