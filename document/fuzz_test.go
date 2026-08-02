// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func FuzzParseImageOptionsReader(f *testing.F) {
	f.Add([]byte{0xff, 0xd8, 0xff, 0xd9}, "jpg")
	if png, err := hex.DecodeString("89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4890000000a49444154789c6360000002000100ffff03000006000557bfab0d0000000049454e44ae426082"); err == nil {
		f.Add(png, "png")
	}
	f.Add([]byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;"), "gif")
	f.Add([]byte("RIFF\x1a\x00\x00\x00WEBPVP8 \x0e\x00\x00\x00\x10\x00\x00\x9d\x01\x2a\x01\x00\x01\x00\x02\x00\x34\x25\xa4\x00\x03p\x00\xfe\xfb\xfdP\x00"), "webp")
	f.Add([]byte("not an image"), "png")
	f.Fuzz(func(t *testing.T, input []byte, imageType string) {
		_, _, _ = parseImageOptionsReader(ImageOptions{ImageType: imageType}, bytes.NewReader(input), 1, defaultCompressionLevel(), "")
	})
}

func FuzzAppendEscapedPDFCellText(f *testing.F) {
	f.Add("plain text")
	f.Add("(paren) \\ slash")
	f.Add("\x00\x01\n\r")
	f.Fuzz(func(t *testing.T, input string) {
		_ = appendEscapedPDFCellText(nil, input)
	})
}
