// Copyright 2015 - António Meireles  <antonio.meireles@reformi.st>
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

package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/codeskyblue/go-sh"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	pullCmd = &cobra.Command{
		Use:     "pull",
		Aliases: []string{"get", "fetch"},
		Short:   "pull a CoreOS image from upstream",
		Run:     pullCommand,
	}
)

func pullCommand(cmd *cobra.Command, args []string) {
	viper.BindPFlags(cmd.Flags())
	vm := &SessionContext.data[0]
	vm.setChannel(viper.GetString("channel"))
	vm.setVersion(viper.GetString("version"))
	vm.lookupImage(viper.GetBool("force"))
}

func init() {
	pullCmd.Flags().String("channel", "alpha",
		"CoreOS channel")
	pullCmd.Flags().String("version", "latest",
		"CoreOS version")
	pullCmd.Flags().BoolP("force", "f", false,
		"override local image, if any")

	RootCmd.AddCommand(pullCmd)
}

func (vm *VMInfo) findLatestUpstream() (version string, err error) {
	upstream := fmt.Sprintf("http://%s.%s/%s",
		vm.Channel, "release.core-os.net", "amd64-usr/current/version.txt")
	signature := "COREOS_VERSION="
	response, err := http.Get(upstream)
	// we're probably offline
	if response != nil {
		defer response.Body.Close()
	}
	if err != nil {
		return version, err
	}
	s := bufio.NewScanner(response.Body)
	s.Split(bufio.ScanLines)
	for s.Scan() {
		line := s.Text()
		if strings.HasPrefix(line, signature) {
			version = strings.TrimPrefix(line, signature)
			return version, err
		}
	}
	// shouldn 't happen ever. will be treated as if offline'
	return version, fmt.Errorf("version not found parsing %s (!)", upstream)
}
func (vm *VMInfo) lookupImage(override bool) {
	var err error
	var isLocal bool
	local := getLocalImages()
	l := local[vm.Channel]

	if SessionContext.debug {
		fmt.Printf("checking CoreOS %s/%s\n", vm.Channel, vm.Version)
	}
	if vm.Version == "latest" {
		vm.Version, err = vm.findLatestUpstream()
		// as we're probably offline
		if err != nil {
			if len(l) == 0 {
				log.Fatalln("offline and not a single locally image",
					"available for", vm.Channel, "channel.")
			}
			vm.Version = l[l.Len()-1].String()
		}
	}
	for _, i := range l {
		if vm.Version == i.String() {
			isLocal = true
			break
		}
	}
	if isLocal {
		if !override {
			if SessionContext.debug {
				fmt.Println("    -", vm.Version, "already downloaded.")
			}
			return
		}
	}

	root := fmt.Sprintf("http://%s.release.core-os.net/amd64-usr/%s/",
		vm.Channel, vm.Version)
	prefix := "coreos_production_pxe"
	files := []string{fmt.Sprintf("%s.vmlinuz", prefix),
		fmt.Sprintf("%s_image.cpio.gz", prefix)}

	for _, j := range files {
		src := fmt.Sprintf("%s%s", root, j)
		downloadAndVerify(src)
	}
}

func downloadAndVerify(t string) {
	f := wget(t)
	sig := wget(fmt.Sprintf("%s.sig", t))

	dir := filepath.Dir(f)
	fn := filepath.Base(f)

	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		log.Fatalln(err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Fatalln(err)
		}
		if err := os.RemoveAll(dir); err != nil {
			log.Fatalln(err)
		}
	}()

	if _, err = exec.LookPath("gpg"); err != nil {
		log.Println("'gpg' not found in PATH.",
			"Unable to verify downloaded image's autenticity.")
	} else {
		verify := sh.NewSession()

		verify.SetEnv("GNUPGHOME", tmpDir)
		verify.SetEnv("GPG_LONG_ID", GPGLongID)
		verify.SetEnv("GPG_KEY", GPGKey)
		verify.ShowCMD = false

		verify.Command("gpg", "--batch", "--quiet",
			"--import").SetInput(GPGKey).CombinedOutput()
		out, err := verify.Command("gpg", "--batch", "--trusted-key", GPGLongID,
			"--verify", sig, f).CombinedOutput()
		legit := fmt.Sprintf("%s %s", "Good signature from \"CoreOS Buildbot",
			"(Offical Builds) <buildbot@coreos.com>\" [ultimate]")
		if err != nil || !strings.Contains(string(out), legit) {
			log.Fatalln("gpg key verification failed for", t)
		}
	}
	if strings.HasSuffix(t, "cpio.gz") {
		oemdir := filepath.Join(dir, "./usr/share/oem/")
		oembindir := filepath.Join(oemdir, "./bin/")
		if err = os.MkdirAll(oembindir, 0755); err != nil {
			log.Fatalln(err)
		}
		if err := ioutil.WriteFile(filepath.Join(oemdir,
			"cloud-config.yml"), []byte(CoreOEMsetup), 0644); err != nil {
			log.Fatalln(err)
		}
		if err := ioutil.WriteFile(filepath.Join(oembindir,
			"coreos-setup-environment"),
			[]byte(CoreOEMsetupEnv), 0755); err != nil {
			log.Fatalln(err)
		}

		oem := sh.NewSession()
		oem.SetDir(dir)
		if out, err := oem.Command("gzip",
			"-dc", fn).Command("cpio",
			"-idv").CombinedOutput(); err != nil {
			log.Fatalln(out, err)
		}
		if out, err := oem.Command("find",
			"usr", "etc", "usr.squashfs").Command("cpio",
			"-oz", "-H", "newc", "-O", f).CombinedOutput(); err != nil {
			log.Fatalln(out, err)
		}
	}
	dest := fmt.Sprintf("%s/images/%s/%s", SessionContext.configDir,
		SessionContext.data[0].Channel, SessionContext.data[0].Version)
	if err = os.MkdirAll(dest, 0755); err != nil {
		log.Fatalln(err)
	}
	if err = os.Rename(f, fmt.Sprintf("%s/%s", dest, fn)); err != nil {
		log.Fatalln(err)
	}
	if SessionContext.hasPowers {
		if err := fixPerms(dest); err != nil {
			log.Fatalln(err)
		}
	}
}
