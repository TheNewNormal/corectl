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
	"encoding/binary"
	"fmt"
	"os"

	"github.com/helm/helm/log"
	"github.com/vbatts/qcow2"
)

var ErrFileIsNotQCOW2 = fmt.Errorf("File doesn't appear to be a qcow one")

// adapted from github.com/vbatts/qcow2/cmd/qcow2-info/main.go
func ValidateQcow2(fh *os.File) (err error) {
	var size int

	buf := make([]byte, qcow2.V2HeaderSize)

	if size, err = fh.Read(buf); err != nil {
		return
	}

	if size >= qcow2.V2HeaderSize && bytes.Compare(buf[:4], qcow2.Magic) != 0 {
		log.Debug("%q: Does not appear to be qcow file %#v %#v",
			fh.Name(), buf[:4], qcow2.Magic)
		return ErrFileIsNotQCOW2
	}

	q := qcow2.Header{
		Version:               qcow2.Version(be32(buf[4:8])),
		BackingFileOffset:     be64(buf[8:16]),
		BackingFileSize:       be32(buf[16:20]),
		ClusterBits:           be32(buf[20:24]),
		Size:                  be64(buf[24:32]),
		CryptMethod:           qcow2.CryptMethod(be32(buf[32:36])),
		L1Size:                be32(buf[36:40]),
		L1TableOffset:         be64(buf[40:48]),
		RefcountTableOffset:   be64(buf[48:56]),
		RefcountTableClusters: be32(buf[56:60]),
		NbSnapshots:           be32(buf[60:64]),
		SnapshotsOffset:       be64(buf[64:72]),
		HeaderLength:          72, // v2 this is a standard length
	}

	if q.Version == 3 {
		if size, err = fh.Read(buf[:qcow2.V3HeaderSize]); err != nil {
			return fmt.Errorf("(qcow2) error validating %q: %s",
				fh.Name(), err)
		}
		if size < qcow2.V3HeaderSize {
			return fmt.Errorf("(qcow2) error validating %q: short read",
				fh.Name())
		}

		q.IncompatibleFeatures = be32(buf[0:8])
		q.CompatibleFeatures = be32(buf[8:16])
		q.AutoclearFeatures = be32(buf[16:24])
		q.RefcountOrder = be32(buf[24:28])
		q.HeaderLength = be32(buf[28:32])
	}
	if log.IsDebugging {
		log.Info("%#v\n", q)
		log.Info("IncompatibleFeatures: %b\n", q.IncompatibleFeatures)
		log.Info("CompatibleFeatures: %b\n", q.CompatibleFeatures)
	}
	// Process the extension header data
	buf = make([]byte, q.HeaderLength)
	if size, err = fh.Read(buf); err != nil {
		return fmt.Errorf("(qcow2) error validating %q: %s", fh.Name(), err)
	}
	if size < q.HeaderLength {
		return fmt.Errorf("(qcow2) error validating %q: short read", fh.Name())
	}
	return
}

func be32(b []byte) int {
	return int(binary.BigEndian.Uint32(b))
}

func be64(b []byte) int64 {
	return int64(binary.BigEndian.Uint64(b))
}
