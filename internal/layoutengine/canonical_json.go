// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package layoutengine

//go:generate sh -c "cd ../../tools && go run ./canonical-json-gen -output ../internal/layoutengine/canonical_json_projection_gen.go"

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"hash"
	"strconv"
	"unicode/utf8"
)

// canonicalJSONHash hashes the exact bytes produced by json.Marshal without
// retaining Marshal's copied result buffer. Encoder uses the same canonical
// struct encoder and escaping rules, but appends one newline; the writer keeps
// the final byte out of the digest and verifies that it is that newline.
func canonicalJSONHash(value any) ([sha256.Size]byte, error) {
	digest := sha256.New()
	writer := trailingNewlineHashWriter{hash: digest}
	if err := json.NewEncoder(&writer).Encode(value); err != nil {
		return [sha256.Size]byte{}, err
	}
	if err := writer.finish(); err != nil {
		return [sha256.Size]byte{}, err
	}
	var result [sha256.Size]byte
	digest.Sum(result[:0])
	return result, nil
}

func canonicalTreeProjectionHash(value CanonicalTreeProjection) ([sha256.Size]byte, error) {
	digest := sha256.New()
	encoder := canonicalHashEncoder{destination: digest}
	encodeCanonicalCanonicalTreeProjection(&encoder, value)
	return encoder.sum(), nil
}

func layoutPlanProjectionHash(value LayoutPlanProjection) ([sha256.Size]byte, error) {
	digest := sha256.New()
	encoder := canonicalHashEncoder{destination: digest}
	encodeCanonicalLayoutPlanProjection(&encoder, value)
	return encoder.sum(), nil
}

const canonicalHashBufferSize = 16 << 10

type canonicalHashEncoder struct {
	destination hash.Hash
	buffer      [canonicalHashBufferSize]byte
	used        int
}

func (encoder *canonicalHashEncoder) flush() {
	if encoder.used == 0 {
		return
	}
	_, _ = encoder.destination.Write(encoder.buffer[:encoder.used])
	encoder.used = 0
}

func (encoder *canonicalHashEncoder) sum() [sha256.Size]byte {
	encoder.flush()
	var result [sha256.Size]byte
	encoder.destination.Sum(result[:0])
	return result
}

func (encoder *canonicalHashEncoder) writeRaw(value string) {
	if len(value) <= len(encoder.buffer)-encoder.used {
		copy(encoder.buffer[encoder.used:], value)
		encoder.used += len(value)
		return
	}
	for len(value) != 0 {
		available := len(encoder.buffer) - encoder.used
		if available == 0 {
			encoder.flush()
			available = len(encoder.buffer)
		}
		count := min(available, len(value))
		copy(encoder.buffer[encoder.used:], value[:count])
		encoder.used += count
		value = value[count:]
	}
}

func (encoder *canonicalHashEncoder) writeByte(value byte) {
	if encoder.used == len(encoder.buffer) {
		encoder.flush()
	}
	encoder.buffer[encoder.used] = value
	encoder.used++
}

func (encoder *canonicalHashEncoder) writeField(first *bool, name string) {
	if !*first {
		encoder.writeByte(',')
	}
	*first = false
	encoder.writeRaw(name)
}

func (encoder *canonicalHashEncoder) reserve(count int) {
	if len(encoder.buffer)-encoder.used < count {
		encoder.flush()
	}
}

func (encoder *canonicalHashEncoder) writeBool(value bool) {
	if value {
		encoder.writeRaw("true")
	} else {
		encoder.writeRaw("false")
	}
}

func (encoder *canonicalHashEncoder) writeInt(value int64) {
	encoder.reserve(20)
	encoded := strconv.AppendInt(encoder.buffer[:encoder.used], value, 10)
	encoder.used = len(encoded)
}

func (encoder *canonicalHashEncoder) writeUint(value uint64) {
	encoder.reserve(20)
	encoded := strconv.AppendUint(encoder.buffer[:encoder.used], value, 10)
	encoder.used = len(encoded)
}

// writeString matches encoding/json's default HTML-safe string encoding. The
// exact byte representation is part of the persisted plan hash contract.
func (encoder *canonicalHashEncoder) writeString(value string) {
	encoder.writeByte('"')
	start := 0
	for index := 0; index < len(value); {
		if current := value[index]; current < utf8.RuneSelf {
			if current >= 0x20 && current != '\\' && current != '"' && current != '<' && current != '>' && current != '&' {
				index++
				continue
			}
			encoder.writeRaw(value[start:index])
			switch current {
			case '\\', '"':
				encoder.writeByte('\\')
				encoder.writeByte(current)
			case '\b':
				encoder.writeRaw(`\b`)
			case '\f':
				encoder.writeRaw(`\f`)
			case '\n':
				encoder.writeRaw(`\n`)
			case '\r':
				encoder.writeRaw(`\r`)
			case '\t':
				encoder.writeRaw(`\t`)
			default:
				encoder.writeRaw(`\u00`)
				encoder.writeByte(canonicalHex[current>>4])
				encoder.writeByte(canonicalHex[current&0x0f])
			}
			index++
			start = index
			continue
		}
		current, size := utf8.DecodeRuneInString(value[index:])
		if current == utf8.RuneError && size == 1 {
			encoder.writeRaw(value[start:index])
			encoder.writeRaw(`\ufffd`)
			index++
			start = index
			continue
		}
		if current == '\u2028' || current == '\u2029' {
			encoder.writeRaw(value[start:index])
			encoder.writeRaw(`\u202`)
			encoder.writeByte(canonicalHex[current&0x0f])
			index += size
			start = index
			continue
		}
		index += size
	}
	encoder.writeRaw(value[start:])
	encoder.writeByte('"')
}

const canonicalHex = "0123456789abcdef"

type trailingNewlineHashWriter struct {
	hash         hash.Hash
	trailing     byte
	haveTrailing bool
	one          [1]byte
}

func (w *trailingNewlineHashWriter) finish() error {
	if !w.haveTrailing || w.trailing != '\n' {
		return errors.New("layoutengine: canonical JSON encoder omitted its final newline")
	}
	w.trailing = 0
	w.haveTrailing = false
	return nil
}

func (w *trailingNewlineHashWriter) Write(value []byte) (int, error) {
	written := len(value)
	if written == 0 {
		return 0, nil
	}
	if w.haveTrailing {
		w.one[0] = w.trailing
		_, _ = w.hash.Write(w.one[:])
	}
	if len(value) > 1 {
		_, _ = w.hash.Write(value[:len(value)-1])
	}
	w.trailing = value[len(value)-1]
	w.haveTrailing = true
	return written, nil
}
