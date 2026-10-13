package archive_test

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	archive "github.com/moby/go-archive"
	"github.com/moby/go-archive/chrootarchive"
	"gotest.tools/v3/assert"
)

// TestDestinationRootContract checks root-relative destinations and rejects
// extraction APIs that would otherwise silently ignore the directory boundary.
func TestDestinationRootContract(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	assert.NilError(t, tw.WriteHeader(&tar.Header{Name: "file", Typeflag: tar.TypeReg, Mode: 0o644}))
	assert.NilError(t, tw.Close())

	for _, api := range []struct {
		name      string
		unpack    func(io.Reader, string, *archive.TarOptions) error
		supported bool
	}{
		{name: "Unpack", unpack: archive.Unpack, supported: true},
		{name: "Untar", unpack: archive.Untar, supported: true},
		{name: "UntarUncompressed", unpack: archive.UntarUncompressed, supported: true},
		{name: "UnpackLayer", unpack: func(r io.Reader, dest string, opts *archive.TarOptions) error {
			_, err := archive.UnpackLayer(dest, r, opts)
			return err
		}},
		{name: "ApplyUncompressedLayer", unpack: func(r io.Reader, dest string, opts *archive.TarOptions) error {
			_, err := archive.ApplyUncompressedLayer(dest, r, opts)
			return err
		}},
		{name: "chrootarchive.Untar", unpack: chrootarchive.Untar},
		{name: "chrootarchive.UntarUncompressed", unpack: chrootarchive.UntarUncompressed},
		{name: "chrootarchive.UntarWithRoot", unpack: func(r io.Reader, dest string, opts *archive.TarOptions) error {
			return chrootarchive.UntarWithRoot(r, dest, opts, dest)
		}},
		{name: "chrootarchive.ApplyUncompressedLayer", unpack: func(r io.Reader, dest string, opts *archive.TarOptions) error {
			_, err := chrootarchive.ApplyUncompressedLayer(dest, r, opts)
			return err
		}},
	} {
		t.Run(api.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			root, err := os.OpenRoot(dir)
			assert.NilError(t, err)
			defer root.Close()
			assert.NilError(t, root.Mkdir("nested", 0o755))
			assert.NilError(t, root.Mkdir("nested/deep", 0o755))
			outside := t.TempDir()
			for _, tc := range []struct {
				name  string
				dest  string
				valid bool
			}{
				{name: "root", dest: "", valid: true},
				{name: "dot", dest: ".", valid: true},
				{name: "nested", dest: "nested", valid: true},
				{name: "deep", dest: "nested/deep", valid: true},
				{name: "missing", dest: "missing"},
				{name: "parent escape", dest: filepath.Join("..", filepath.Base(outside))},
				{name: "absolute", dest: outside},
			} {
				t.Run(tc.name, func(t *testing.T) {
					err := api.unpack(bytes.NewReader(buf.Bytes()), tc.dest, &archive.TarOptions{NoLchown: true, DestinationRoot: root})
					switch {
					case !api.supported:
						assert.ErrorContains(t, err, "DestinationRoot is not supported")
					case !tc.valid:
						assert.Assert(t, err != nil, "destination must be an existing directory within DestinationRoot")
					default:
						assert.NilError(t, err)
						_, err = root.Stat(filepath.Join(tc.dest, "file"))
						assert.NilError(t, err)
					}
					_, err = root.Stat(".")
					assert.NilError(t, err, "extraction must not close DestinationRoot")
				})
			}
			_, err = os.Stat(filepath.Join(outside, "file"))
			assert.Assert(t, os.IsNotExist(err), "extraction must not write outside DestinationRoot")
		})
	}
}
