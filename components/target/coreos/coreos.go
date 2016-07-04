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
	"time"

	"github.com/TheNewNormal/corectl/components/common/assets"
)

const latestImageBreackage = "2016-06-25T00:00:00WET"

func LatestImageBreackage() (t time.Time) {
	t, _ = time.Parse("2006-01-02T15:04:05MST", latestImageBreackage)
	return
}

// CoreOS default Channels
var Channels = []string{"alpha", "beta", "stable"}

const defaultChannel = "alpha"
const defaultVersion = "latest"

var (
	GPGLongID            = "50E0885593D2DCB4"
	GPGKey               = assets.Contents("target/coreos/CoreOSkey.public")
	CoreOEMsharedHomedir = assets.Contents("target/coreos/homedir.yml.tmpl")
	CoreOSIgnitionTmpl   = assets.Contents("target/coreos/corectl.ignition.yml.tmpl")
)
