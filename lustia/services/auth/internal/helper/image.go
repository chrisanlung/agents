package helper

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg" // register JPEG decoder
	_ "image/png"  // register PNG decoder
	"io"
	"net/http"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/disintegration/imaging"
)

const (
	// MaxImageDimension is the hard ceiling on any image side (px).
	// The resize step brings final output to ≤1024 in either dimension.
	MaxImageDimension = 4096

	// resizeMaxDim is the target max for the longest side after re-encode.
	resizeMaxDim = 1024

	// sniffBytes is how many bytes we read to detect the MIME type.
	sniffBytes = 512

	// decodeConfigLimit limits the bytes consumed by image.DecodeConfig.
	decodeConfigLimit = 1 << 20 // 1 MiB
)

// allowedMIMETypes lists the MIME types accepted by the upload pipeline.
var allowedMIMETypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// ProcessUpload validates, transforms, and re-encodes an uploaded image.
//
// Pipeline (ADR 0011 §2.4):
//  1. Read first 512 bytes — MIME sniff.
//  2. Reject anything outside {image/jpeg, image/png, image/webp}.
//  3. Wrap sniffed prefix + remaining reader via io.MultiReader.
//  4. image.DecodeConfig inside io.LimitReader(1 MiB) — validates structure +
//     dimensions; kills polyglot files and most decompression bombs at header
//     level without full decode.
//  5. Full decode + re-encode via imaging.Fit(1024, 1024). JPEG → JPEG@85,
//     PNG → PNG, WebP → JPEG@85 (imaging has no WebP encoder).
//  6. Return stripped bytes + final MIME.
//
// EXIF is stripped automatically: imaging.Decode does not retain metadata.
func ProcessUpload(r io.Reader, maxBytes int64) (stripped []byte, mime string, err error) {
	// --- Step 1: MIME sniff ---
	sniffBuf := make([]byte, sniffBytes)
	n, err := io.ReadFull(r, sniffBuf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, "", fmt.Errorf("%w: cannot read image data", constants.ErrInvalidImageFormat)
	}
	sniffBuf = sniffBuf[:n]

	detected := http.DetectContentType(sniffBuf)
	// http.DetectContentType may return "image/webp" or the generic
	// "application/octet-stream" for WebP in older stdlib versions.
	// Normalise by also accepting octet-stream for now only if the
	// first 4 bytes are "RIFF" (WebP container). Otherwise reject.
	if detected == "application/octet-stream" {
		if len(sniffBuf) >= 12 && string(sniffBuf[0:4]) == "RIFF" && string(sniffBuf[8:12]) == "WEBP" {
			detected = "image/webp"
		}
	}

	// --- Step 2: reject disallowed MIME types ---
	if !allowedMIMETypes[detected] {
		return nil, "", fmt.Errorf("%w: got %q", constants.ErrInvalidImageFormat, detected)
	}

	// --- Step 3: reassemble reader ---
	full := io.MultiReader(bytes.NewReader(sniffBuf), r)

	// --- Step 4: structural validation via DecodeConfig ---
	// Use a LimitReader so malformed length headers cannot cause huge allocs.
	limR := io.LimitReader(full, decodeConfigLimit)
	// We need to be able to re-read after DecodeConfig, so buffer everything.
	bodyBytes, err := io.ReadAll(limR)
	if err != nil {
		return nil, "", fmt.Errorf("%w: cannot read image body", constants.ErrInvalidImageFormat)
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, "", fmt.Errorf("%w: cannot decode image config: %v", constants.ErrInvalidInput, err)
	}
	if cfg.Width > MaxImageDimension || cfg.Height > MaxImageDimension {
		return nil, "", fmt.Errorf("%w: %dx%d exceeds %dpx ceiling",
			constants.ErrImageDimensionsTooLarge, cfg.Width, cfg.Height, MaxImageDimension)
	}

	// Check raw size against caller-supplied maxBytes.
	if int64(len(bodyBytes)) > maxBytes {
		return nil, "", constants.ErrImageTooLarge
	}

	// --- Step 5: full decode + re-encode (strips EXIF) ---
	img, err := imaging.Decode(bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, "", fmt.Errorf("%w: cannot decode image: %v", constants.ErrInvalidInput, err)
	}

	// Resize to fit within resizeMaxDim × resizeMaxDim (preserves aspect ratio).
	if img.Bounds().Dx() > resizeMaxDim || img.Bounds().Dy() > resizeMaxDim {
		img = imaging.Fit(img, resizeMaxDim, resizeMaxDim, imaging.Lanczos)
	}

	// Determine output format. WebP → JPEG (imaging has no WebP encoder).
	var outMIME string
	var outFmt imaging.Format
	switch detected {
	case "image/png":
		outMIME = "image/png"
		outFmt = imaging.PNG
	default: // image/jpeg, image/webp → JPEG
		outMIME = "image/jpeg"
		outFmt = imaging.JPEG
	}

	var buf bytes.Buffer
	encOpts := []imaging.EncodeOption{}
	if outFmt == imaging.JPEG {
		encOpts = append(encOpts, imaging.JPEGQuality(85))
	}
	if err := imaging.Encode(&buf, img, outFmt, encOpts...); err != nil {
		return nil, "", fmt.Errorf("image encode failed: %w", err)
	}

	return buf.Bytes(), outMIME, nil
}

// ImageExt returns the canonical file extension for a MIME type.
// Returns "jpg" for image/jpeg, "png" for image/png, "jpg" as fallback.
func ImageExt(mime string) string {
	switch mime {
	case "image/png":
		return "png"
	default:
		return "jpg"
	}
}
