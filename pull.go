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

	"github.com/codegangsta/cli"
	"github.com/codeskyblue/go-sh"
)

//
func pullAction(c *cli.Context) {
	SessionContext.data.setChannel(c.String("channel"))
	SessionContext.data.setVersion(c.String("version"))

	SessionContext.data.lookupImage()
}

//
func imageFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{
			Name:   "version",
			Value:  "latest",
			Usage:  "CoreOS version",
			EnvVar: "COREOS_VERSION,VERSION",
		},
		cli.StringFlag{
			Name:   "channel",
			Value:  "alpha",
			Usage:  "CoreOS channel",
			EnvVar: "COREOS_CHANNEL,CHANNEL",
		},
	}
}

func (vm *VMInfo) findLatestUpstream() (version string, err error) {
	upstream := fmt.Sprintf("http://%s.%s/%s",
		vm.Channel, "release.core-os.net", "amd64-usr/current/version.txt")
	signature := "COREOS_VERSION="
	response, err := http.Get(upstream)
	// we're probably offline
	if got(response) {
		defer response.Body.Close()
	}
	if got(err) {
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
func (vm *VMInfo) lookupImage() {
	var err error
	var isLocal bool
	local := getLocalImages()
	l := local[vm.Channel]

	fmt.Printf("checking CoreOS %s/%s\n", vm.Channel, vm.Version)
	if vm.Version == "latest" {
		vm.Version, err = vm.findLatestUpstream()
		// as we're probably offline
		if got(err) {
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
		fmt.Println("    -", vm.Version, "already downloaded.")
	} else {
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
}

func downloadAndVerify(t string) {
	f := wget(t)
	sig := wget(fmt.Sprintf("%s.sig", t))

	dir := filepath.Dir(f)
	fn := filepath.Base(f)

	tmpDir, err := ioutil.TempDir("", "")
	if got(err) {
		log.Fatalln(err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); got(err) {
			log.Fatalln(err)
		}
		if err := os.RemoveAll(dir); got(err) {
			log.Fatalln(err)
		}
	}()

	if _, err = exec.LookPath("gpg"); got(err) {
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
		if got(err) || !strings.Contains(string(out), legit) {
			log.Fatalln("gpg key verification failed for", t)
		}
	}
	if strings.HasSuffix(t, "cpio.gz") {
		oemdir := filepath.Join(dir, "./usr/share/oem/")
		oembindir := filepath.Join(oemdir, "./bin/")
		if err = os.MkdirAll(oembindir, 0755); got(err) {
			log.Fatalln(err)
		}
		if err := ioutil.WriteFile(filepath.Join(oemdir,
			"cloud-config.yml"), []byte(CoreOEMsetup), 0644); got(err) {
			log.Fatalln(err)
		}
		if err := ioutil.WriteFile(filepath.Join(oembindir,
			"coreos-setup-environment"),
			[]byte(CoreOEMsetupEnv), 0755); got(err) {
			log.Fatalln(err)
		}

		oem := sh.NewSession()
		oem.SetDir(dir)
		if out, err := oem.Command("gzip",
			"-dc", fn).Command("cpio",
			"-idv").CombinedOutput(); got(err) {
			log.Fatalln(out, err)
		}
		if out, err := oem.Command("find",
			"usr", "etc", "usr.squashfs").Command("cpio",
			"-oz", "-H", "newc", "-O", f).CombinedOutput(); got(err) {
			log.Fatalln(out, err)
		}
	}

	dest := fmt.Sprintf("%s/images/%s/%s", SessionContext.configDir,
		SessionContext.data.Channel, SessionContext.data.Version)
	if err = os.MkdirAll(dest, 0755); got(err) {
		log.Fatalln(err)
	}
	if err = os.Rename(f, fmt.Sprintf("%s/%s", dest, fn)); got(err) {
		log.Fatalln(err)
	}
	if SessionContext.hasPowers {
		if err := fixPerms(dest); got(err) {
			log.Fatalln(err)
		}
	}
}
