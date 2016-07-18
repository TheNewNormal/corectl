package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"os"

	"github.com/vbatts/qcow2"
)

func main() {
	flag.Parse()

	for _, arg := range flag.Args() {
		fh, err := os.Open(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERR] %q: %s\n", arg, err)
			os.Exit(1)
		}
		defer fh.Close()

		buf := make([]byte, qcow2.V2HeaderSize)
		size, err := fh.Read(buf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERR] %q: %s\n", arg, err)
			os.Exit(1)
		}
		if size < qcow2.V2HeaderSize {
			fmt.Fprintf(os.Stderr, "[ERR] %q: short read\n", arg)
			os.Exit(1)
		}

		if bytes.Compare(buf[:4], qcow2.Magic) != 0 {
			fmt.Fprintf(os.Stderr, "[ERR] %q: Does not appear to be qcow file %#v %#v\n", arg, buf[:4], qcow2.Magic)
			os.Exit(1)
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
			size, err := fh.Read(buf[:qcow2.V3HeaderSize])
			if err != nil {
				fmt.Fprintf(os.Stderr, "[ERR] %q: %s\n", arg, err)
				os.Exit(1)
			}
			if size < qcow2.V3HeaderSize {
				fmt.Fprintf(os.Stderr, "[ERR] %q: short read\n", arg)
				os.Exit(1)
			}

			q.IncompatibleFeatures = be32(buf[0:8])
			q.CompatibleFeatures = be32(buf[8:16])
			q.AutoclearFeatures = be32(buf[16:24])
			q.RefcountOrder = be32(buf[24:28])
			q.HeaderLength = be32(buf[28:32])
		}
		fmt.Printf("%#v\n", q)
		fmt.Printf("IncompatibleFeatures: %b\n", q.IncompatibleFeatures)
		fmt.Printf("CompatibleFeatures: %b\n", q.CompatibleFeatures)

		// Process the extension header data
		buf = make([]byte, q.HeaderLength)
		size, err = fh.Read(buf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERR] %q: %s\n", arg, err)
			os.Exit(1)
		}
		if size < q.HeaderLength {
			fmt.Fprintf(os.Stderr, "[ERR] %q: short read\n", arg)
			os.Exit(1)
		}
		for {
			t := qcow2.HeaderExtensionType(be32(buf[:4]))
			if t == qcow2.HdrExtEndOfArea {
				break
			}
			exthdr := qcow2.ExtHeader{
				Type: t,
				Size: be32(buf[4:8]),
			}
			// XXX this may need a copy(), so the slice resuse doesn't corrupt
			exthdr.Data = buf[8 : 8+exthdr.Size]
			q.ExtHeaders = append(q.ExtHeaders, exthdr)

			round := exthdr.Size % 8
			buf = buf[8+exthdr.Size+round:]
		}

	}
}

func be32(b []byte) int {
	return int(binary.BigEndian.Uint32(b))
}

func be64(b []byte) int64 {
	return int64(binary.BigEndian.Uint64(b))
}
