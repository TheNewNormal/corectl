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
	"os"
	"os/exec"
	"strings"

	"github.com/deis/pkg/log"
	// looks to be the new upstream
	"github.com/keybase/go-ps"

	"github.com/genevera/corectl/components/host/session"
)

func nfsSetup() (err error) {
	const exportsF = "/etc/exports"
	var (
		buf, bufN []byte
		shared    bool
		oldSigA   = "/Users -network 192.168.64.0 " +
			"-mask 255.255.255.0 -alldirs -mapall="
		oldSigB = fmt.Sprintf("%v -network %v -mask %v -alldirs -mapall=",
			session.Caller.HomeDir, session.Caller.Network.Base(),
			session.Caller.Network.Mask)
		signature = fmt.Sprintf("%v -network %v -mask %v -alldirs "+
			"-maproot=root:wheel", session.Caller.HomeDir,
			session.Caller.Network.Base(), session.Caller.Network.Mask)
		exportSet = func() (ok bool) {
			for _, line := range strings.Split(string(buf), "\n") {
				if strings.HasPrefix(line, signature) {
					ok = true
				}
				if !strings.HasPrefix(line, oldSigA) &&
					!strings.HasPrefix(line, oldSigB) {
					bufN = append(bufN, []byte(line+"\n")...)
				} else {
					bufN = append(bufN, []byte("\n")...)
				}
			}
			return
		}
		nfsIsRunning = func() bool {
			all, _ := ps.Processes()
			for _, p := range all {
				if strings.HasSuffix(p.Executable(), "nfsd") {
					return true
				}
			}
			return false
		}()
		exportsCheck = func(previous []byte) (err error) {
			var out []byte
			if out, err = exec.Command("nfsd", "-F",
				exportsF, "checkexports").Output(); err != nil {
				err = fmt.Errorf("unable to validate %s ('%v')\n"+
					"keeping original contents ('%v')",
					exportsF, string(out), string(previous))
				// getting back to where we were
				ioutil.WriteFile(exportsF, previous, os.ModeAppend)
			}
			return
		}
	)
	// check if /etc/exports exists, and if not create an empty one
	if _, err = os.Stat(exportsF); os.IsNotExist(err) {
		if err = ioutil.WriteFile(exportsF, []byte(""), 0644); err != nil {
			return
		}
	}

	if buf, err = ioutil.ReadFile(exportsF); err != nil {
		return
	}

	if shared = exportSet(); !shared {
		if err = ioutil.WriteFile(exportsF, append(bufN,
			[]byte(signature+"\n")...), os.ModeAppend); err != nil {
			return
		}
	}

	if err = exportsCheck(buf); err != nil {
		return
	}

	if nfsIsRunning {
		if !shared {
			if err = exec.Command("nfsd", "update").Run(); err != nil {
				return fmt.Errorf("unable to update nfs "+
					"service definitions... (%v)", err)
			}
			log.Info("'%s' now available to VMs' network via nfs",
				session.Caller.HomeDir)
		} else {
			log.Info("'%s' was already available to VMs' network via nfs",
				session.Caller.HomeDir)
		}
	} else {
		if err = exec.Command("nfsd", "start").Run(); err != nil {
			return fmt.Errorf("unable to start NFS service... (%v)", err)
		}
		log.Info("nfs service started in order for '%s' to be "+
			"made available to VMs' network", session.Caller.HomeDir)
	}
	return
}
