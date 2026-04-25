package helper_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const maxBytes = 10 * 1024 * 1024 // 10 MiB for tests

// makeJPEG returns a minimal valid JPEG image as bytes.
func makeJPEG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 255, G: 128, B: 0, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// makePNG returns a minimal valid PNG image as bytes.
func makePNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func TestProcessUpload_JPEG_HappyPath(t *testing.T) {
	t.Parallel()

	data := makeJPEG(200, 300)
	stripped, mime, err := helper.ProcessUpload(bytes.NewReader(data), maxBytes)
	require.NoError(t, err)
	assert.Equal(t, "image/jpeg", mime)
	assert.NotEmpty(t, stripped)
}

func TestProcessUpload_PNG_HappyPath(t *testing.T) {
	t.Parallel()

	data := makePNG(200, 200)
	stripped, mime, err := helper.ProcessUpload(bytes.NewReader(data), maxBytes)
	require.NoError(t, err)
	assert.Equal(t, "image/png", mime)
	assert.NotEmpty(t, stripped)
}

func TestProcessUpload_LargeImage_GetsResized(t *testing.T) {
	t.Parallel()

	// 2000×1000 should resize to ≤1024 on the longest side.
	data := makeJPEG(2000, 1000)
	stripped, mime, err := helper.ProcessUpload(bytes.NewReader(data), maxBytes)
	require.NoError(t, err)
	assert.Equal(t, "image/jpeg", mime)

	// Decode output and check dimensions.
	img, _, err := image.Decode(bytes.NewReader(stripped))
	require.NoError(t, err)
	bounds := img.Bounds()
	assert.LessOrEqual(t, bounds.Dx(), 1024)
	assert.LessOrEqual(t, bounds.Dy(), 1024)
}

func TestProcessUpload_RejectsNonImage(t *testing.T) {
	t.Parallel()

	// Plain text masquerading as an image.
	data := []byte("this is not an image at all")
	_, _, err := helper.ProcessUpload(bytes.NewReader(data), maxBytes)
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrInvalidImageFormat)
}

func TestProcessUpload_RejectsDimensionBomb(t *testing.T) {
	t.Parallel()

	// 5000×5000 exceeds the 4096 ceiling.
	// Build manually: we need a PNG with fake large dimensions in its header.
	// Instead, use a real 5000×5000 but that would be slow; skip if OOM concern.
	// Use a synthetic approach: make a small 10×10 JPEG and check the path
	// with a dimension just over 4096 using a fake PNG header is complex.
	// The simplest reliable approach: create actual 100×100 but verify the
	// dimension check path via the constant.
	// This test validates the code path by using exactly MaxImageDimension+1.
	// That would allocate ~16 MiB; skip in short mode.
	if testing.Short() {
		t.Skip("skipping large dimension test in short mode")
	}

	oversized := makeJPEG(helper.MaxImageDimension+1, 100)
	_, _, err := helper.ProcessUpload(bytes.NewReader(oversized), maxBytes)
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrImageDimensionsTooLarge)
}

func TestProcessUpload_RejectsOversizedBytes(t *testing.T) {
	t.Parallel()

	data := makeJPEG(100, 100)
	// Allow only 1 byte → must fail with ErrImageTooLarge.
	_, _, err := helper.ProcessUpload(bytes.NewReader(data), 1)
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrImageTooLarge)
}

func TestProcessUpload_EXIFStripped(t *testing.T) {
	t.Parallel()

	// Build a JPEG with a fake EXIF APP1 segment containing "GPS" marker.
	// A minimal JPEG: SOI + APP1(EXIF with GPS text) + SOF + ... just use
	// a real encode and check the output does not contain the GPS marker string.
	// The simplest: encode normally and verify "GPS" does not appear in output
	// (imaging re-encode produces a clean JFIF with no EXIF).
	// We embed a fake EXIF-like block manually before the actual JPEG.

	base := makeJPEG(50, 50)
	// Inject fake EXIF: splice in an APP1 marker after SOI.
	// SOI = FF D8. We insert FF E1 + length(2 bytes big-endian) + "Exif\0\0GPS..." after SOI.
	exifPayload := []byte("Exif\x00\x00GPS coordinates here")
	app1 := make([]byte, 0, 4+len(exifPayload))
	app1 = append(app1, 0xFF, 0xE1) // APP1 marker
	segLen := uint16(len(exifPayload) + 2)
	app1 = append(app1, byte(segLen>>8), byte(segLen))
	app1 = append(app1, exifPayload...)

	// base[0:2] = SOI (FF D8). Insert app1 right after SOI.
	withEXIF := make([]byte, 0, len(base)+len(app1))
	withEXIF = append(withEXIF, base[0:2]...)   // SOI
	withEXIF = append(withEXIF, app1...)         // fake APP1 EXIF
	withEXIF = append(withEXIF, base[2:]...)     // rest of JPEG

	stripped, _, err := helper.ProcessUpload(bytes.NewReader(withEXIF), maxBytes)
	require.NoError(t, err)

	// After re-encode by imaging, the APP1/EXIF block must be gone.
	assert.NotContains(t, string(stripped), "GPS")
}

func TestImageExt(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "png", helper.ImageExt("image/png"))
	assert.Equal(t, "jpg", helper.ImageExt("image/jpeg"))
	assert.Equal(t, "jpg", helper.ImageExt("image/webp"))
	assert.Equal(t, "jpg", helper.ImageExt(""))
}
