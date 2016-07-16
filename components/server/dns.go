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

//  adapted from github.com/kubernetes/minikube/pkg/localkube/dns.go
//

package server

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/TheNewNormal/corectl/components/host/session"
	"github.com/miekg/dns"
	backendetcd "github.com/skynetservices/skydns/backends/etcd"
	skymetrics "github.com/skynetservices/skydns/metrics"
	skydns "github.com/skynetservices/skydns/server"
	"k8s.io/minikube/pkg/util"
)

var (
	RecursiveNameServers = []string{
		"8.8.8.8:53",
		"8.8.4.4:53",
	}
	LocalDomainName = "coreos.local"
)

type DNSServer struct {
	sky           runner
	dnsServerAddr *net.UDPAddr
	done          chan struct{}
}

func (d *ServerContext) NewDNSServer(root,
	serverAddress string, ns []string) (err error) {
	var (
		dnsAddress *net.UDPAddr
		skyConfig  = &skydns.Config{
			DnsAddr:     serverAddress,
			Domain:      root,
			Nameservers: ns,
		}
	)
	if dnsAddress, err = net.ResolveUDPAddr("udp", serverAddress); err != nil {
		return
	}

	skydns.SetDefaults(skyConfig)

	backend := backendetcd.NewBackend(d.EtcdClient, context.Background(),
		&backendetcd.Config{
			Ttl:      skyConfig.Ttl,
			Priority: skyConfig.Priority,
		})
	skyServer := skydns.New(backend, skyConfig)

	// setup so prometheus doesn't run into nil
	skymetrics.Metrics()

	d.DNSServer = &DNSServer{
		sky:           skyServer,
		dnsServerAddr: dnsAddress,
	}
	// make host visible to the VMs by Name
	if err = d.DNSServer.addRecord("corectld",
		session.Caller.Network.Address); err != nil {
		return
	}
	d.DNSServer.Start()
	return
}

func (dns *DNSServer) Start() {
	if dns.done != nil {
		fmt.Fprint(os.Stderr, util.Pad("DNS server already started"))
		return
	}

	dns.done = make(chan struct{})

	go util.Until(dns.sky.Run, os.Stderr, "skydns", 1*time.Second, dns.done)

}

func (dns *DNSServer) Stop() {
	teardownService()

	// closing chan will prevent servers from restarting but will not kill
	// running server
	close(dns.done)

}

// runner starts a server returning an error if it stops.
type runner interface {
	Run() error
}

func teardownService() {
	Daemon.DNSServer.rmRecord("corectld", session.Caller.Network.Address)
}

func invertDomain(in string) (out string) {
	s := strings.Split(in, ".")
	for x := len(s) - 1; x >= 0; x-- {
		out += s[x] + "/"
	}
	out = strings.TrimSuffix(out, "/")
	return
}

func (d *DNSServer) addRecord(hostName string, ip string) (err error) {
	var r string
	path := fmt.Sprintf("/skydns/%s/%s", invertDomain(LocalDomainName),
		strings.Replace(hostName, ".", "/", -1))

	if _, err = Daemon.EtcdClient.Set(context.Background(), path,
		"{\"host\":\""+ip+"\"}", nil); err != nil {
		return
	}
	// reverse
	hostName = hostName + "." + LocalDomainName
	if r, err = dns.ReverseAddr(ip); err != nil {
		return
	}
	_, err = Daemon.EtcdClient.Set(context.Background(),
		"/skydns/"+r, "{\"host\":\""+hostName+"\"}", nil)
	return
}
func (d *DNSServer) rmRecord(hostName string, ip string) (err error) {
	var r string
	path := fmt.Sprintf("/skydns/%s/%s", invertDomain(LocalDomainName),
		strings.Replace(hostName, ".", "/", -1))
	if _, err =
		Daemon.EtcdClient.Delete(context.Background(), path, nil); err != nil {
		return
	}
	// reverse
	hostName = hostName + "." + LocalDomainName
	if r, err = dns.ReverseAddr(ip); err != nil {
		return
	}
	_, err = Daemon.EtcdClient.Delete(context.Background(),
		"/skydns/"+r, nil)
	return
}
