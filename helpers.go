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
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// returns true if payload not nil/empty
func got(in interface{}) bool {
	switch i := in.(type) {
	case string:
		if i != "" {
			return true
		}
	case []string:
		if len(i) > 0 {
			return true
		}
	case []VMInfo:
		if len(i) > 0 {
			return true
		}
	// bool is a corner case, and this is useless anyway when interface is bool
	// just in case we just return original value...
	case bool:
		return i
	case *http.Response:
		if i != nil {
			return true
		}
	default:
		if i != nil {
			return true
		}
	}
	return false
}

// returns true if payload nil/empty
func empty(failure interface{}) bool {
	return !got(failure)
}

// (recursively) fix permissions on path
func fixPerms(path string) error {
	u, _ := strconv.Atoi(SessionContext.uid)
	g, _ := strconv.Atoi(SessionContext.gid)

	action := func(p string, _ os.FileInfo, _ error) error {
		return os.Chown(p, u, g)
	}
	return filepath.Walk(path, action)
}

// downloads url to disk and returns its location
func wget(url string) (f string) {
	tmpDir, err := ioutil.TempDir("", "coreos")
	if got(err) {
		log.Fatalln(err)
	}
	cleanup := func() {
		if err := os.RemoveAll(tmpDir); got(err) {
			log.Println(err)
		}
	}
	tmpDir += "/"
	tok := strings.Split(url, "/")
	f = tmpDir + tok[len(tok)-1]
	fmt.Println("    - downloading", url)
	output, err := os.Create(f)
	if got(err) {
		cleanup()
		log.Fatalf("%s (%s)", f, err)
	}
	defer output.Close()
	r, err := http.Get(url)
	if got(r) {
		defer r.Body.Close()
	}
	if got(err) {
		cleanup()
		log.Fatalln("remote system seems to be offline...")
	}
	if r.StatusCode != 200 {
		cleanup()
		log.Fatalf("requested URL (%s) doesn't seems to exist...", url)
	}
	n, err := io.Copy(output, r.Body)
	if got(err) {
		cleanup()
		log.Fatalf("%s (%s)", err, err)
	}
	fmt.Println("      -", n, "bytes downloaded.")
	return f
}
