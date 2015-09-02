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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/spf13/viper"
)

// (recursively) fix permissions on path
func fixPerms(path string) error {
	u, _ := strconv.Atoi(SessionContext.uid)
	g, _ := strconv.Atoi(SessionContext.gid)

	action := func(p string, _ os.FileInfo, _ error) error {
		return os.Chown(p, u, g)
	}
	return filepath.Walk(path, action)
}

func pSlice(plain string) []string {
	var sliced []string
	for _, x := range viper.GetStringSlice(plain) {
		strip := strings.Replace(strings.Replace(x, "]", "", -1), "[", "", -1)
		for _, y := range strings.Split(strip, ",") {
			sliced = append(sliced, y)
		}
	}
	return sliced
}

// downloads url to disk and returns its location
func wget(url string) (f string) {
	tmpDir, err := ioutil.TempDir("", "coreos")
	if err != nil {
		log.Fatalln(err)
	}
	cleanup := func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Println(err)
		}
	}
	tmpDir += "/"
	tok := strings.Split(url, "/")
	f = tmpDir + tok[len(tok)-1]
	if SessionContext.debug {
		fmt.Println("    - downloading", url)
	}
	output, err := os.Create(f)
	if err != nil {
		cleanup()
		log.Fatalf("%s (%s)", f, err)
	}
	defer output.Close()
	r, err := http.Get(url)
	if r != nil {
		defer r.Body.Close()
	}
	if err != nil {
		cleanup()
		log.Fatalln("remote system seems to be offline...")
	}
	if r.StatusCode != 200 {
		cleanup()
		log.Fatalf("requested URL (%s) doesn't seems to exist...", url)
	}
	n, err := io.Copy(output, r.Body)
	if err != nil {
		cleanup()
		log.Fatalf("%s (%s)", err, err)
	}
	if SessionContext.debug {
		fmt.Println("      -", n, "bytes downloaded.")
	}
	return f
}

// sshKeyGen creates a one-time ssh public and private key pair
func sshKeyGen() (string, string, error) {
	secret, err := rsa.GenerateKey(rand.Reader, 2014)
	if err != nil {
		return "", "", err
	}

	secretDer := x509.MarshalPKCS1PrivateKey(secret)
	secretBlk := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   secretDer,
	}

	privateKey := string(pem.EncodeToMemory(&secretBlk))

	public, _ := ssh.NewPublicKey(&secret.PublicKey)
	publicFormated := string(ssh.MarshalAuthorizedKey(public))

	return privateKey, publicFormated, nil
}
