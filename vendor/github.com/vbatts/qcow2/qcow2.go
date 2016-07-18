package qcow2

var (
	// Magic is the front of the file fingerprint
	Magic = []byte{0x51, 0x46, 0x49, 0xFB}

	// V2HeaderSize is the image header at the beginning of the file
	V2HeaderSize = 72

	// V3HeaderSize is directly following the v2 header, up to 104
	V3HeaderSize = 104 - V2HeaderSize
)

type (
	// Version number of this image. Valid versions are 2 or 3
	Version int

	// CryptMethod is whether no encryption (0), or AES encryption (1)
	CryptMethod int

	// HeaderExtensionType indicators the the entries in the optional header area
	HeaderExtensionType int
)

const (
	HdrExtEndOfArea         HeaderExtensionType = 0x00000000
	HdrExtBackingFileFormat HeaderExtensionType = 0xE2792ACA
	HdrExtFeatureNameTable  HeaderExtensionType = 0x6803f857 // TODO needs processing for feature name table
	// any thing else is "other" and can be ignored
)

func (qcm CryptMethod) String() string {
	if qcm == 1 {
		return "AES"
	}
	return "none"
}

type Header struct {
	// magic [:4]
	Version               Version     // [4:8]
	BackingFileOffset     int64       // [8:16]
	BackingFileSize       int         // [16:20]
	ClusterBits           int         // [20:24]
	Size                  int64       // [24:32]
	CryptMethod           CryptMethod // [32:36]
	L1Size                int         // [36:40]
	L1TableOffset         int64       // [40:48]
	RefcountTableOffset   int64       // [48:56]
	RefcountTableClusters int         // [56:60]
	NbSnapshots           int         // [60:64]
	SnapshotsOffset       int64       // [64:72]

	// v3
	IncompatibleFeatures int // [72:80] bitmask
	CompatibleFeatures   int // [80:88] bitmask
	AutoclearFeatures    int // [88:96] bitmask
	RefcountOrder        int // [96:100]
	HeaderLength         int // [100:104]

	// Header extensions
	ExtHeaders []ExtHeader
}

type ExtHeader struct {
	Type HeaderExtensionType
	Size int
	Data []byte
}
