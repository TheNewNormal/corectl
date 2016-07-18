package qcow2

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"os"
	"testing"
)

/*
qemu-img create -f qcow2 file.qcow2 100m
sudo modprobe nbd max_part=63
sudo qemu-nbd -f qcow2 -c /dev/nbd0 file.qcow2
sudo mkfs.ext2 /dev/nbd0
mkdir -p file/
sudo mount /dev/nbd0 file/
sudo umount file/
sudo qemu-nbd -d /dev/nbd0
sudo qemu-img snapshot -c base file.qcow2
sudo qemu-nbd -f qcow2 -c /dev/nbd0 file.qcow2
sudo mount /dev/nbd0 file
echo Howdy | sudo dd of=file/hello.txt
sudo umount file/
sudo qemu-nbd -d /dev/nbd0
sudo qemu-img snapshot -c hello file.qcow2
sudo qemu-nbd -f qcow2 -c /dev/nbd0 file.qcow2
sudo mount /dev/nbd0 file
sudo rm file/hello.txt
sudo umount file/
sudo qemu-nbd -d /dev/nbd0
ls -lsh ./file.qcow2
# 4.9M -rw-r--r--. 1 vbatts vbatts 5.1M Sep  3 13:38 file.qcow2
qcow2-info ./file.qcow2
# image: ./file.qcow2
# file format: qcow2
# virtual size: 100M (104857600 bytes)
# disk size: 4.8M
# cluster_size: 65536
# Snapshot list:
# ID        TAG                 VM SIZE                DATE       VM CLOCK
# 1         base                      0 2015-09-03 13:36:55   00:00:00.000
# 2         hello                     0 2015-09-03 13:38:07   00:00:00.000
# Format specific information:
#    compat: 1.1
#    lazy refcounts: false
#    refcount bits: 16
#    corrupt: false
gzip -9 file.qcow2 > file.qcow2.gz
*/
var testQcowFile = "./testdata/file.qcow2.gz"

func TestHeader(t *testing.T) {
	f, err := os.Open(testQcowFile)
	if err != nil {
		t.Fatal(err)
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	rdr := bufio.NewReader(gz)

	buf := make([]byte, V2HeaderSize)
	size, err := rdr.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if size < V2HeaderSize {
		t.Fatal(err)
	}

	if bytes.Compare(buf[:4], Magic) != 0 {
		t.Fatalf("[ERR] Does not appear to be qcow file %#v %#v\n", buf[:4], Magic)
	}

	q := Header{
		Version:               Version(be32(buf[4:8])),
		BackingFileOffset:     be64(buf[8:16]),
		BackingFileSize:       be32(buf[16:20]),
		ClusterBits:           be32(buf[20:24]),
		Size:                  be64(buf[24:32]),
		CryptMethod:           CryptMethod(be32(buf[32:36])),
		L1Size:                be32(buf[36:40]),
		L1TableOffset:         be64(buf[40:48]),
		RefcountTableOffset:   be64(buf[48:56]),
		RefcountTableClusters: be32(buf[56:60]),
		NbSnapshots:           be32(buf[60:64]),
		SnapshotsOffset:       be64(buf[64:72]),
		HeaderLength:          72, // v2 this is a standard length
	}

	if q.Version == 3 {
		size, err := rdr.Read(buf[:V3HeaderSize])
		if err != nil {
			t.Fatal(err)
		}
		if size < V3HeaderSize {
			t.Fatalf("short read")
		}

		q.IncompatibleFeatures = be32(buf[0:8])
		q.CompatibleFeatures = be32(buf[8:16])
		q.AutoclearFeatures = be32(buf[16:24])
		q.RefcountOrder = be32(buf[24:28])
		q.HeaderLength = be32(buf[28:32])
	}
	t.Logf("%#v", q)

	// Process the extension header data
	buf = make([]byte, q.HeaderLength)
	size, err = rdr.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if size < q.HeaderLength {
		t.Fatalf("short read")
	}
	for {
		t := HeaderExtensionType(be32(buf[:4]))
		if t == HdrExtEndOfArea {
			break
		}
		exthdr := ExtHeader{
			Type: t,
			Size: be32(buf[4:8]),
		}
		// XXX this may need a copy(), so the slice resuse doesn't corrupt
		exthdr.Data = buf[8 : 8+exthdr.Size]
		q.ExtHeaders = append(q.ExtHeaders, exthdr)

		round := exthdr.Size % 8
		buf = buf[8+exthdr.Size+round:]
	}

	// TODO at this point we can do some assertions on the `q` values
}

func be32(b []byte) int {
	return int(binary.BigEndian.Uint32(b))
}

func be64(b []byte) int64 {
	return int64(binary.BigEndian.Uint64(b))
}
