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
	"net"
	"net/http"
	"strconv"
	"strings"
	"text/template"

	"github.com/TheNewNormal/corectl/components/host/session"
	"github.com/TheNewNormal/corectl/components/target/coreos"
	"github.com/coreos/fuze/config"
	"github.com/coreos/go-systemd/unit"
	"github.com/coreos/ignition/config/types"
	"github.com/deis/pkg/log"
	"github.com/gorilla/mux"
)

var httpServices = mux.NewRouter()

type corectlTmpl struct {
	SetupRoot, PersistentRoot, SharedHomedir   bool
	CorectlVersion, CorectldEndpoint           string
	NetworkdGateway, DomainName, Hostname      string
	NFShomedirPath, NFShomedirPathEscaped      string
	SSHAuthorizedKeys, UserProvidedFuzeConfigs []string
}

func httpServiceSetup() {
	httpServices.HandleFunc("/{uuid}/ignition/append/{id}",
		httpInstanceUserProvidedIgnitionConfigs)
	httpServices.HandleFunc("/{uuid}/ignition/default/config",
		httpInstanceDefaultIgnitionConfig)
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
		if vm.CloudConfig.Location == "" {
			httpError(w, http.StatusPreconditionFailed)
		} else if vm.CloudConfig.Contents == nil {
			httpError(w, http.StatusInternalServerError)
		} else {
			vars := strings.NewReplacer("__vm.Name__", vm.Name)
			w.Write([]byte(vars.Replace(string(vm.CloudConfig.Contents))))
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

func httpInstanceUserProvidedIgnitionConfigs(w http.ResponseWriter,
	r *http.Request) {
	if acceptableRequest(r, w) {
		vm := Daemon.Active[mux.Vars(r)["uuid"]]
		ign, err := strconv.Atoi(mux.Vars(r)["id"])
		if err != nil {
			httpError(w, http.StatusPreconditionFailed)
		}
		if len(vm.IgnitionFuzeConfigs) == 0 {
			httpError(w, http.StatusPreconditionFailed)
		} else if len(vm.IgnitionFuzeConfigs) < ign+1 {
			httpError(w, http.StatusPreconditionFailed)
		} else {
			if out, err := processIgnitionTemplate(r, vm,
				string(vm.IgnitionFuzeConfigs[ign].Contents)); err != nil {
				log.Err("%v", err.Error())
				httpError(w, http.StatusInternalServerError)
			} else {
				w.Write([]byte(append(out, '\n')))
			}
		}
	}
}
func httpInstanceDefaultIgnitionConfig(w http.ResponseWriter, r *http.Request) {
	if acceptableRequest(r, w) {
		vm := Daemon.Active[mux.Vars(r)["uuid"]]
		if out, err := processIgnitionTemplate(r, vm,
			coreos.CoreOSIgnitionTmpl); err != nil {
			log.Err("%v", err.Error())
			httpError(w, http.StatusInternalServerError)
		} else {
			w.Write([]byte(append(out, '\n')))
			if !isLoopback(remoteIP(r.RemoteAddr)) {
				Daemon.DNSServer.addRecord(vm.Name, remoteIP(r.RemoteAddr))
			}
		}
	}
}

func processIgnitionTemplate(r *http.Request, vm *VMInfo,
	original string) (processed []byte, err error) {
	var (
		rendered bytes.Buffer
		cfgIn    types.Config
		setup    = corectlTmpl{
			vm.FormatRoot,
			vm.PersistentRoot,
			vm.SharedHomedir,
			Daemon.Meta.Version,
			vm.endpoint(),
			session.Caller.Network.Address,
			LocalDomainName,
			vm.Name,
			session.Caller.HomeDir,
			unit.UnitNamePathEscape(session.Caller.HomeDir),
			[]string{vm.InternalSSHkey},
			[]string{},
		}
	)
	if vm.SSHkey != "" {
		setup.SSHAuthorizedKeys = append(setup.SSHAuthorizedKeys, vm.SSHkey)
	}
	for _, fz := range vm.IgnitionFuzeConfigs {
		setup.UserProvidedFuzeConfigs =
			append(setup.UserProvidedFuzeConfigs, fz.Location)
	}
	t, _ := template.New("").Parse(original)
	if err = t.Execute(&rendered, setup); err != nil {
		return
	}

	log.Info(rendered.String())

	if cfgIn, err = config.ParseAsV2_0_0(rendered.Bytes()); err != nil {
		return
	}
	processed, err = json.MarshalIndent(&cfgIn, "", "  ")
	return
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
