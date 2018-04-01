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

package session

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/genevera/corectl/components/target/coreos"
	"github.com/genevera/corectl/release"
	"github.com/bugsnag/osext"
	"github.com/deis/pkg/log"
	"github.com/spf13/viper"
)

var (
	// Caller ...
	Caller               *Context
	ErrServerUnreachable = fmt.Errorf("Failed to connect to server\n" +
		"Please check your connection settings and ensure that 'corectld' " +
		"is running.\n")
)

// Network ...
type Network struct {
	Address string
	Mask    string
}

// Context ...
type Context struct {
	Privileged    bool          `json:"-"`
	Meta          *release.Info `json:"-"`
	CmdLine       *viper.Viper  `json:"-"`
	ServerAddress string
	*user.User    `json:"User"`
	*Network      `json:"Network"`
}

// New session context
func New() (ctx *Context, err error) {
	var (
		usr                 *user.User
		isSuperUser         bool
		netMask, netAddress []byte
	)

	// viper & cobra
	rawArgs := viper.New()
	rawArgs.SetEnvPrefix("COREOS")
	rawArgs.AutomaticEnv()
	euid := os.Getuid()

	if euid == 0 {
		usr = nil
		isSuperUser = true
	} else if usr, err = user.Current(); err != nil {
		return
	}

	return &Context{
		isSuperUser,
		&release.Info{
			release.Version,
			time.Now(),
			os.Getpid(),
			release.BuildDate,
			runtime.Version(),
			runtime.GOOS,
			runtime.GOARCH,
		},
		rawArgs,
		"127.0.0.1:2511",
		usr,
		&Network{
			strings.TrimSpace(string(netAddress)),
			strings.TrimSpace(string(netMask)),
		},
	}, err
}

// Base IP
func (n *Network) Base() string {
	return net.ParseIP(n.Address).Mask(
		net.IPMask(net.ParseIP(n.Mask).To4())).String()
}

// Debug ...
func (ctx *Context) Debug() bool {
	return ctx.CmdLine.GetBool("debug")
}

// JSON ...
func (ctx *Context) JSON() bool {
	return ctx.CmdLine.GetBool("json")
}

// ConfigDir ...
func (ctx *Context) ConfigDir() string {
	return path.Join(ctx.HomeDir, "/.coreos/")
}

// ImageStore ...
func (ctx *Context) ImageStore() string {
	return path.Join(ctx.ConfigDir(), "/images/")
}

// RunDir ...
func (ctx *Context) RunDir() string {
	return path.Join(ctx.ConfigDir(), "/running/")
}

// TmpDir ...
func (ctx *Context) TmpDir() string {
	return path.Join(ctx.ConfigDir(), "/tmp/")
}

// EtcDir ...
func (ctx *Context) EtcDir() string {
	return path.Join(ctx.ConfigDir(), "/embedded.etcd/")
}

// NormalizeOnDiskLayout ...
func (ctx *Context) NormalizeOnDiskLayout() (err error) {
	// first run
	for _, i := range coreos.Channels {
		if err =
			os.MkdirAll(path.Join(ctx.ImageStore(), i),
				0755); err != nil {
			return
		}
	}
	for _, i := range []string{ctx.RunDir(), ctx.TmpDir(), ctx.EtcDir()} {
		if err = os.MkdirAll(i, 0755); err != nil {
			return
		}
	}
	// usually image updates
	if !ctx.Privileged {
		return
	}
	u, _ := strconv.Atoi(ctx.Uid)
	g, _ := strconv.Atoi(ctx.Gid)

	do := func(p string,
		_ os.FileInfo, _ error) error {
		tty := path.Base(p)
		if tty == "tty" {
			return os.Remove(p)
		}
		// to fix previous sins
		if err := os.Chmod(p, 0755); err != nil {
			return err
		}
		return os.Chown(p, u, g)
	}
	return filepath.Walk(ctx.ConfigDir(), do)
}

func (ctx *Network) SetContext() (err error) {
	var (
		netMask, netAddress []byte
		cmdL                = []string{
			"defaults", "read",
			"/Library/Preferences/SystemConfiguration/com.apple.vmnet.plist",
		}
	)
	if netAddress, err = exec.Command(cmdL[0],
		append(cmdL[1:], "Shared_Net_Address")...).Output(); err != nil {
		err = nil
		log.Warn("%v \"%v %v %v\" %v ...",
			"unable to run", cmdL[0], cmdL[1], cmdL[2], "Shared_Net_Address")
		log.Warn("... assuming macOS default value (192.168.64.1)")
		netAddress = []byte("192.168.64.1")
	}

	if netMask, err = exec.Command(cmdL[0],
		append(cmdL[1:], "Shared_Net_Mask")...).Output(); err != nil {
		err = nil
		log.Warn("%v \"%v %v %v\" %v ...",
			"unable to run", cmdL[0], cmdL[1], cmdL[2], "Shared_Net_Mask")
		log.Warn("... assuming macOS default value (255.255.255.0)")
		netMask = []byte("255.255.255.0")
	}
	ctx.Address = strings.TrimSpace(string(netAddress))
	ctx.Mask = strings.TrimSpace(string(netMask))
	return
}

func Executable() string {
	s, _ := osext.Executable()
	return s
}

func AppName() string {
	s, _ := osext.Executable()
	return filepath.Base(s)
}

func ExecutableFolder() string {
	s, _ := osext.ExecutableFolder()
	return s
}
