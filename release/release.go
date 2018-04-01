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

package release

import (
	"fmt"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/google/go-github/github"
)

var (
	// Version of running blob
	// -ldflags "-X github.com/genevera/corectl/release/Version=
	//            `git describe --abbrev=6 --dirty=-unreleased --always --tags`"
	Version string
	// BuildDate of running blob
	BuildDate string
	// ShortBanner ...
	ShortBanner = "CoreOS over macOS made simple."
	// Banner ...
	Banner = fmt.Sprintf("%s <%s>\n%s\n", ShortBanner,
		"http://github.com/genevera/corectl",
		"Copyright (c) 2015-2016, António Meireles")
	// Info ...
)

type Info struct {
	Version string
	Started time.Time
	Pid     int
	Built   string
	Runtime string
	GOOS    string
	GOARCH  string
}

// LatestVersion returns the latest upstream release.
func LatestVersion() (version string, err error) {
	var latest *github.RepositoryRelease
	// if err we're probably in offline mode
	if latest, _, err =
		github.NewClient(nil).Repositories.GetLatestRelease("genevera",
			"corectl"); err == nil {
		version = *latest.TagName
	}
	return
}

func (i *Info) PrettyPrint(extended bool) {
	lo := "2006-01-02T15:04:05MST"
	lf := "Mon Jan 02 15:04:05 MST 2006"
	stamp, _ := time.Parse(lo, i.Built)
	fmt.Printf(" Version:\t%v\n Go Version:\t%v\n "+
		"Built:\t\t%v\n OS/Arch:\t%v/%v\n",
		strings.TrimPrefix(i.Version, "v"), i.Runtime, stamp.Format(lf),
		i.GOOS, i.GOARCH)
	if extended {
		fmt.Printf("\n Pid:\t\t%v\n Uptime:\t%v\n\n",
			i.Pid,
			humanize.Time(i.Started))
	}
}
