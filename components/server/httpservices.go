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
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"strings"

	"github.com/TheNewNormal/corectl/components/host/session"
	"github.com/TheNewNormal/corectl/components/target/coreos"
	"github.com/coreos/fuze/config"
	"github.com/coreos/go-systemd/unit"
	"github.com/gorilla/mux"
)

var httpServices = mux.NewRouter()

func httpServiceSetup() {
	httpServices.HandleFunc("/{uuid}/ignition", httpInstanceIgnitionConfig)
	httpServices.HandleFunc("/{uuid}/cloud-config", httpInstanceCloudConfig)
	httpServices.HandleFunc("/{uuid}/homedir", httpInstanceHomedirMountConfig)
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
		} else if t, err := ioutil.ReadFile(vm.CloudConfig); err != nil {
			httpError(w, http.StatusInternalServerError)
		} else {
			w.Write(t)
		}
	}
}

func httpInstanceIgnitionConfig(w http.ResponseWriter, r *http.Request) {
	if acceptableRequest(r, w) {
		vm := Daemon.Active[mux.Vars(r)["uuid"]]
		mods := strings.NewReplacer(
			"__vm.InternalSSHkey__", vm.InternalSSHkey,
			"__vm.Name__", vm.Name,
			"__corectl.Version__", Daemon.Meta.Version)
		if cfgIn, err := config.ParseAsV2_0_0(
			[]byte(mods.Replace(coreos.CoreOSIgnitionTmpl))); err != nil {
			httpError(w, http.StatusInternalServerError)
		} else if i, err := json.MarshalIndent(&cfgIn, "", "  "); err != nil {
			httpError(w, http.StatusInternalServerError)
		} else {
			w.Write([]byte(append(i, '\n')))
		}
	}
}

func httpInstanceHomedirMountConfig(w http.ResponseWriter, r *http.Request) {
	if acceptableRequest(r, w) {
		mods := strings.NewReplacer("((server))", session.Caller.Address,
			"((path))", session.Caller.HomeDir,
			"((path_escaped))", unit.UnitNamePathEscape(session.Caller.HomeDir))
		w.Write([]byte(mods.Replace(coreos.CoreOEMsharedHomedir)))
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

