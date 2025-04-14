package compression

import "testing"

func TestExtension(t *testing.T) {
	tests := []struct {
		compression Compression
		extension   string
	}{
		{compression: -1, extension: ""},
		{compression: None, extension: "tar"},
		{compression: Bzip2, extension: "tar.bz2"},
		{compression: Gzip, extension: "tar.gz"},
		{compression: Xz, extension: "tar.xz"},
		{compression: Zstd, extension: "tar.zst"},
	}
	for _, tc := range tests {
		if actual := tc.compression.Extension(); actual != tc.extension {
			t.Errorf("expected %s extension got %s", tc.extension, actual)
		}
	}
}

func TestDetectCompressionZstd(t *testing.T) {
	// test zstd compression without skippable frames.
	compressedData := []byte{
		0x28, 0xb5, 0x2f, 0xfd, // magic number of Zstandard frame: 0xFD2FB528
		0x04, 0x00, 0x31, 0x00, 0x00, // frame header
		0x64, 0x6f, 0x63, 0x6b, 0x65, 0x72, // data block "docker"
		0x16, 0x0e, 0x21, 0xc3, // content checksum
	}
	compression := Detect(compressedData)
	if compression != Zstd {
		t.Fatal("Unexpected compression")
	}
	// test zstd compression with skippable frames.
	hex := []byte{
		0x50, 0x2a, 0x4d, 0x18, // magic number of skippable frame: 0x184D2A50 to 0x184D2A5F
		0x04, 0x00, 0x00, 0x00, // frame size
		0x5d, 0x00, 0x00, 0x00, // user data
		0x28, 0xb5, 0x2f, 0xfd, // magic number of Zstandard frame: 0xFD2FB528
		0x04, 0x00, 0x31, 0x00, 0x00, // frame header
		0x64, 0x6f, 0x63, 0x6b, 0x65, 0x72, // data block "docker"
		0x16, 0x0e, 0x21, 0xc3, // content checksum
	}
	compression = Detect(hex)
	if compression != Zstd {
		t.Fatal("Unexpected compression")
	}
}
