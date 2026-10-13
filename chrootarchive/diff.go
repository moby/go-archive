package chrootarchive

import (
	"errors"
	"io"

	"github.com/moby/go-archive"
)

// ApplyLayer parses a diff in the standard layer format from `layer`,
// and applies it to the directory `dest`. The stream `layer` can only be
// uncompressed.
// Returns the size in bytes of the contents of the layer.
func ApplyLayer(dest string, layer io.Reader) (size int64, err error) {
	return applyLayerHandler(dest, layer, &archive.TarOptions{}, true)
}

// ApplyUncompressedLayer parses a diff in the standard layer format from
// `layer`, and applies it to the directory `dest`. The stream `layer`
// can only be uncompressed.
// Returns the size in bytes of the contents of the layer.
func ApplyUncompressedLayer(dest string, layer io.Reader, options *archive.TarOptions) (int64, error) {
	if options != nil && options.DestinationRoot != nil {
		return 0, errors.New("DestinationRoot is not supported by chrootarchive")
	}
	return applyLayerHandler(dest, layer, options, false)
}
