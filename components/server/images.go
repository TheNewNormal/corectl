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
	"bytes"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"

	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/clearsign"

	"github.com/TheNewNormal/corectl/components/host/session"
	"github.com/TheNewNormal/corectl/components/target/coreos"

	"github.com/deis/pkg/log"
	"github.com/rakyll/pb"

	"github.com/blang/semver"
)

func localImages() (local map[string]semver.Versions, err error) {
	local = make(map[string]semver.Versions, 0)

	for _, channel := range coreos.Channels {
		releasesStore := []os.FileInfo{}
		dir := path.Join(session.Caller.ChannelsDir(), channel)
		available := semver.Versions{}

		if releasesStore, err = ioutil.ReadDir(dir); err != nil {
			return
		}
		for _, rev := range releasesStore {
			if rev.Mode()&os.ModeSymlink == os.ModeSymlink {
				var ok = true

				for _, f := range []string{".vmlinuz", "_image.cpio.gz"} {
					fn := path.Join(dir, rev.Name(), "coreos_production_pxe"+f)
					if _, err = os.Stat(fn); err != nil {
						ok = false
						log.Warn("%v missing - %v/%v ignored",
							fn, channel, rev.Name())
						break
					}
				}
				if ok {
					v := semver.Version{}
					if v, err = semver.Make(rev.Name()); err != nil {
						return
					}
					available = append(available, v)
				} else {
					// XXX
					if err =
						os.RemoveAll(path.Join(dir, rev.Name())); err != nil {
						return
					}
				}
			}
		}
		semver.Sort(available)
		local[channel] = available
	}
	all, allReleases := semver.Versions{}, []os.FileInfo{}
	if allReleases, err = ioutil.ReadDir(session.Caller.ReleasesDir()); err != nil {
		return
	}
	for _, r := range allReleases {
		if r.IsDir() && r.Name() != "channels" {
			v := semver.Version{}
			if v, err = semver.Make(r.Name()); err != nil {
				return
			}
			all = append(all, v)
		}
	}
	semver.Sort(all)
	local["releases"] = all
	return
}

// PullImage ...
func PullImage(channel, version string,
	override, preferLocal bool) (v string, err error) {
	var (
		available   bool
		allChannels map[string]semver.Versions
		latest      string
	)

	if allChannels, err = localImages(); err != nil {
		return version, err
	}
	local := allChannels[channel]
	if version == "latest" {
		if preferLocal == true && len(local) > 0 {
			version = local[local.Len()-1].String()
		} else {
			if latest, err =
				coreos.LatestUpstream(channel); err != nil || len(latest) == 0 {
				// as we're probably offline
				if len(local) == 0 {
					err = fmt.Errorf("offline and not a single locally image"+
						"available for '%s' channel", channel)
					return
				}
				version = local[local.Len()-1].String()
			} else {
				version = latest
			}
		}
	}

	for _, i := range local {
		if version == i.String() {
			available = true
			break
		}
	}
	if !available {
		for _, i := range allChannels["releases"] {
			if version == i.String() {
				available = true
				if err = os.Symlink(path.Join(session.Caller.ReleasesDir(), version),
					path.Join(session.Caller.ChannelsDir(), channel, version)); err != nil {
					return
				}
				log.Info("reusing %v as it got promoted to %v",
					version, channel)

				break
			}
		}
	}
	if available {
		if !override {
			log.Debug("%s/%s already available on your system", channel, version)
			return version, err
		} else {
			// tell server that this image become unavailable in the meantime
			if _, err = RPCQuery("RemoveImage", &RPCquery{
				Input: []string{channel, version}}); err != nil {
				return
			}
		}
	}
	return localize(channel, version)
}

func localize(channel, version string) (b string, err error) {
	var files map[string]string
	destination := path.Join(session.Caller.ReleasesDir(), version)

	if err = os.MkdirAll(destination, 0755); err != nil {
		return version, err
	}
	if files, err = downloadAndVerify(channel, version); err != nil {
		return version, err
	}
	defer func() {
		for _, location := range files {
			if e := os.RemoveAll(path.Dir(location)); e != nil {
				log.Err(e.Error())
			}
		}
	}()
	for fn, location := range files {
		if err = os.Rename(location, path.Join(destination, fn)); err != nil {
			return version, err
		}
	}
	if err = os.Symlink(destination, path.Join(session.Caller.ChannelsDir(),
		channel, version)); err != nil {
		return version, err
	}
	if err = session.Caller.NormalizeOnDiskLayout(); err == nil {
		log.Info("%s/%s ready", channel, version)
	}
	return version, err
}
func downloadAndVerify(channel,
	version string) (location map[string]string, err error) {
	var (
		prefix = "coreos_production_pxe"
		root   = fmt.Sprintf("http://%s.release.core-os.net/amd64-usr/%s/",
			channel, version)
		files = []string{fmt.Sprintf("%s.vmlinuz", prefix),
			fmt.Sprintf("%s_image.cpio.gz", prefix)}
		digestFilename = prefix + "_image.cpio.gz.DIGESTS.asc"
		signature      = fmt.Sprintf("%s%s", root, digestFilename)

		tmpDir, bzHashSHA512     string
		output                   *os.File
		digestRaw, longIDdecoded []byte
		r, digest                *http.Response
		longIDdecodedInt         uint64
		keyring                  openpgp.EntityList
		check                    *openpgp.Entity
		re                       = regexp.MustCompile(
			`(?m)(?P<method>(SHA1|SHA512)) HASH(?:\r?)\n(?P<hash>` +
				`.[^\s]*)\s*(?P<file>[\w\d_\.]*)`)
		keymap = make(map[string]int)
	)
	location = make(map[string]string)
	log.Info("downloading and verifying %s/%v", channel, version)
	for _, target := range files {
		url := fmt.Sprintf("%s%s", root, target)

		if tmpDir, err = ioutil.TempDir(session.Caller.TmpDir(), "coreos"); err != nil {
			return
		}
		defer func() {
			if err != nil {
				if e := os.RemoveAll(tmpDir); e != nil {
					log.Err(e.Error())
				}
			}
		}()
		token := strings.Split(url, "/")
		fileName := token[len(token)-1]
		pack := path.Join(tmpDir, fileName)
		if _, err = http.Head(url); err != nil {
			return
		}
		if digest, err = http.Get(signature); err != nil {
			return
		}
		defer digest.Body.Close()
		switch digest.StatusCode {
		case http.StatusOK, http.StatusNoContent:
		default:
			return location, fmt.Errorf("failed fetching %s: HTTP status: %s",
				signature, digest.Status)
		}
		if digestRaw, err = ioutil.ReadAll(digest.Body); err != nil {
			return
		}
		if longIDdecoded, err = hex.DecodeString(coreos.GPGLongID); err != nil {
			return
		}
		longIDdecodedInt = binary.BigEndian.Uint64(longIDdecoded)
		log.Debug("Trusted hex key id %s is decimal %d",
			coreos.GPGLongID, longIDdecoded)
		if keyring, err = openpgp.ReadArmoredKeyRing(
			bytes.NewBufferString(coreos.GPGKey)); err != nil {
			return
		}
		messageClear, _ := clearsign.Decode(digestRaw)
		digestTxt := string(messageClear.Bytes)
		messageClearRdr := bytes.NewReader(messageClear.Bytes)
		if check, err =
			openpgp.CheckDetachedSignature(keyring, messageClearRdr,
				messageClear.ArmoredSignature.Body); err != nil {
			return location, fmt.Errorf("Signature check for DIGESTS failed.")
		}
		if check.PrimaryKey.KeyId == longIDdecodedInt {
			log.Debug("Trusted key id %d matches keyid %d",
				longIDdecodedInt, longIDdecodedInt)
		}
		if _, ok := location[digestFilename]; !ok {
			if err = ioutil.WriteFile(path.Join(tmpDir, digestFilename),
				digestRaw, 0644); err != nil {
				return
			}
			location[digestFilename] = path.Join(tmpDir, digestFilename)
		}
		log.Debug("DIGESTS signature OK. ")

		for index, name := range re.SubexpNames() {
			keymap[name] = index
		}

		matches := re.FindAllStringSubmatch(digestTxt, -1)

		for _, match := range matches {
			if match[keymap["file"]] == fileName {
				if match[keymap["method"]] == "SHA512" {
					bzHashSHA512 = match[keymap["hash"]]
				}
			}
		}

		sha512h := sha512.New()

		if r, err = http.Get(url); err != nil {
			return
		}
		defer r.Body.Close()

		switch r.StatusCode {
		case http.StatusOK, http.StatusNoContent:
		default:
			return location, fmt.Errorf("failed fetching %s: HTTP status: %s",
				signature, r.Status)
		}

		bar := pb.New(int(r.ContentLength)).SetUnits(pb.U_BYTES)
		bar.Start()

		if output, err = os.Create(pack); err != nil {
			return
		}
		defer output.Close()

		writer := io.MultiWriter(sha512h, bar, output)
		io.Copy(writer, r.Body)
		bar.Finish()
		if hex.EncodeToString(sha512h.Sum([]byte{})) != bzHashSHA512 {
			return location, fmt.Errorf("SHA512 hash verification failed for %s",
				fileName)
		}
		log.Info("SHA512 hash for %s OK", fileName)

		location[fileName] = pack
	}
	return location, err
}
