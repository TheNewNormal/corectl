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
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"text/template"

	"github.com/genevera/corectl/components/host/session"
	"github.com/genevera/corectl/components/target/coreos"
	"github.com/coreos/fuze/config"
	"github.com/coreos/go-systemd/unit"
	"github.com/deis/pkg/log"
	"github.com/gorilla/mux"
)

var httpServices = mux.NewRouter()

type corectlTmpl struct {
	SetupRoot, PersistentRoot, SharedHomedir bool
	CorectlVersion, CorectldEndpoint         string
	NetworkdGateway, NetworkdDns, Hostname   string
	SSHAuthorizedKeys                        []string
	NFShomedirPath, NFShomedirPathEscaped    string
}

func httpServiceSetup() {
	httpServices.HandleFunc("/{uuid}/ignition", httpInstanceIgnitionConfig)
	httpServices.HandleFunc("/{uuid}/cloud-config", httpInstanceCloudConfig)
	httpServices.HandleFunc("/{uuid}/ping", httpInstanceCallback)
	httpServices.HandleFunc("/{uuid}/NotIsolated",
		httpInstanceExternalConnectivity)
}

func remoteIP(s string) string {
	return strings.Split(s, ":")[0]
}

func isLoopback(s string) bool {
	if strings.HasPrefix(s, "127.") {
		return true
	}
	return false
}

func httpError(w http.ResponseWriter, status int) {
	w.WriteHeader(status)
	w.Write(nil)
}

func acceptableRequest(r *http.Request, w http.ResponseWriter) bool {
	var (
		vmNetwork = net.IPNet{
			IP:   net.ParseIP(session.Caller.Address),
			Mask: net.IPMask(net.ParseIP(session.Caller.Mask).To4()),
		}
		addr   = remoteIP(r.RemoteAddr)
		status = http.StatusUnauthorized
	)
	if isLoopback(addr) || vmNetwork.Contains(net.ParseIP(addr)) {
		if _, ok := Daemon.Active[mux.Vars(r)["uuid"]]; ok {
			return true
		}
		status = http.StatusNotFound
	}
	httpError(w, status)
	return false
}

func httpInstanceCloudConfig(w http.ResponseWriter, r *http.Request) {
	if acceptableRequest(r, w) {
		vm := Daemon.Active[mux.Vars(r)["uuid"]]
		if vm.CloudConfig == "" || vm.CClocation != Local {
			httpError(w, http.StatusPreconditionFailed)
		} else if vm.cloudConfigContents == nil {
			httpError(w, http.StatusInternalServerError)
		} else {
			vars := strings.NewReplacer("__vm.Name__", vm.Name)
			w.Write([]byte(vars.Replace(string(vm.cloudConfigContents))))
		}
	}
}

func isPortOpen(t string, target string) bool {
	server, err := net.Dial(t, target)
	if err == nil {
		server.Close()
		return true
	}
	return false
}

func httpInstanceIgnitionConfig(w http.ResponseWriter, r *http.Request) {
	if acceptableRequest(r, w) {
		var (
			rendered bytes.Buffer
			vm       = Daemon.Active[mux.Vars(r)["uuid"]]
			setup    = corectlTmpl{
				vm.FormatRoot,
				vm.PersistentRoot,
				vm.SharedHomedir,
				Daemon.Meta.Version,
				vm.endpoint(),
				session.Caller.Network.Address,
				LocalDomainName,
				vm.Name,
				[]string{vm.InternalSSHkey},
				session.Caller.HomeDir,
				unit.UnitNamePathEscape(session.Caller.HomeDir),
			}
		)
		if vm.SSHkey != "" {
			setup.SSHAuthorizedKeys = append(setup.SSHAuthorizedKeys, vm.SSHkey)
		}
		if vm.CloudConfig != "" && vm.CClocation == Local {
			vm.cloudConfigContents, _ = ioutil.ReadFile(vm.CloudConfig)
		}
		t, _ := template.New("").Parse(string(coreos.CoreOSIgnitionTmpl))
		if err := t.Execute(&rendered, setup); err != nil {
			log.Err("==> %v", err.Error())
			httpError(w, http.StatusInternalServerError)
		}

		log.Info(rendered.String())
		if cfgIn, err := config.ParseAsV2_0_0(rendered.Bytes()); err != nil {
			httpError(w, http.StatusInternalServerError)
		} else if i, err := json.MarshalIndent(&cfgIn, "", "  "); err != nil {
			httpError(w, http.StatusInternalServerError)
		} else {
			w.Write([]byte(append(i, '\n')))
			if !isLoopback(remoteIP(r.RemoteAddr)) {
				Daemon.DNSServer.addRecord(vm.Name, remoteIP(r.RemoteAddr))
			}
		}
	}
}

func httpInstanceExternalConnectivity(w http.ResponseWriter, r *http.Request) {
	if acceptableRequest(r, w) {
		if isLoopback(remoteIP(r.RemoteAddr)) {
			httpError(w, http.StatusUnauthorized)
		} else {
			vm := Daemon.Active[mux.Vars(r)["uuid"]]
			w.Write([]byte("ok\n"))
			vm.isolationCheck.Do(func() {
				Daemon.Lock()
				Daemon.Active[vm.UUID].NotIsolated = true
				Daemon.Unlock()
			})
		}
	}
}
func httpInstanceCallback(w http.ResponseWriter, r *http.Request) {
	if acceptableRequest(r, w) {
		if isLoopback(remoteIP(r.RemoteAddr)) {
			httpError(w, http.StatusUnauthorized)
		} else {
			vm := Daemon.Active[mux.Vars(r)["uuid"]]
			w.Write([]byte("ok\n"))
			vm.callBack.Do(func() {
				Daemon.Lock()
				Daemon.Active[vm.UUID].publicIPCh <- remoteIP(r.RemoteAddr)
				Daemon.Unlock()
			})
		}
	}
}
