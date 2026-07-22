// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package layoutengine

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"hash"
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
	if !writer.haveTrailing || writer.trailing != '\n' {
		return [sha256.Size]byte{}, errors.New("layoutengine: canonical JSON encoder omitted its final newline")
	}
	var result [sha256.Size]byte
	digest.Sum(result[:0])
	return result, nil
}

type trailingNewlineHashWriter struct {
	hash         hash.Hash
	trailing     byte
	haveTrailing bool
	one          [1]byte
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
