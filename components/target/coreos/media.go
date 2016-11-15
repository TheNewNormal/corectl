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

package coreos

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"

	"github.com/blang/semver"
	"github.com/deis/pkg/log"
)

// Version validation
func Version(version string) string {
	if version == defaultVersion {
		return version
	}
	if _, err := semver.Make(version); err != nil {
		log.Warn("'%s' is not in a recognizable CoreOS version format. "+
			"Using default ('%s') instead", version, defaultVersion)
		return defaultVersion
	}
	return version
}

// Channel validation
func Channel(name string) string {
	for _, b := range Channels {
		if b == name {
			return b
		}
	}
	log.Warn("'%s' is not a recognizable CoreOS image channel. "+
		"Using default ('%s')", name, defaultChannel)
	return defaultChannel
}

// LatestUpstream returns for the given channel the current shipping version
func LatestUpstream(channel string) (string, error) {
	url := fmt.Sprintf("http://%s.release.core-os.net/"+
		"amd64-usr/current/version.txt", channel)

	response, err := http.Get(url)
	// if err we're probably offline
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK, http.StatusNoContent:
	default:
		return "", fmt.Errorf("failed fetching %s: HTTP status: %s",
			url, response.Status)
	}

	s := bufio.NewScanner(response.Body)
	s.Split(bufio.ScanLines)
	for s.Scan() {
		line := s.Text()
		if eq := strings.LastIndex(line, "COREOS_VERSION="); eq >= 0 {
			if v := strings.Split(line, "=")[1]; len(v) > 0 {
				return v, err
			}
		}
	}
	return "", fmt.Errorf("unable to grab 'COREOS_VERSION' from %s (!)", url)
}
