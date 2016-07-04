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
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/TheNewNormal/corectl/components/host/session"
	"github.com/helm/helm/log"
)

func detectVPN() (utun []string, err error) {
	l, err := net.Interfaces()
	if err != nil {
		return
	}
	for _, f := range l {
		if strings.HasPrefix(f.Name, "utun") {
			utun = append(utun, f.Name)
		}
	}
	return
}

func HandleVPNtunnels() (f func(), err error) {
	var vpnIfs []string

	if vpnIfs, err = detectVPN(); err != nil {
		return
	}

	f = func() {
		if len(vpnIfs) == 0 {
			return
		}
		log.Info("removing custom firewall rules for VPN handling")
		for _, iface := range vpnIfs {
			anchorName := fmt.Sprintf("com.apple/%snat", iface)
			exec.Command("pfctl", "-a", anchorName, "-F", "nat").Output()
		}
	}

	if len(vpnIfs) > 0 {
		log.Info("VPN detected: tweaking host firewall")
		for _, iface := range vpnIfs {
			var ruleFile *os.File
			anchorName := fmt.Sprintf("com.apple/%snat", iface)

			if ruleFile, err = ioutil.TempFile("", "coreos"); err != nil {
				return
			}
			r := fmt.Sprintf("nat on {%s} proto {tcp, udp, icmp} "+
				"from %s/24 to any -> {%s}\n",
				iface, session.Caller.Network.Base(), iface)
			ruleFile.Write([]byte(r))
			ruleFile.Close()
			defer os.RemoveAll(ruleFile.Name())
			if _, err = exec.Command("pfctl",
				"-a", anchorName, "-f", ruleFile.Name()).Output(); err != nil {
				return
			}
		}
	}
	return
}
