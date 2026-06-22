package cid

import (
	"crypto/sha256"
	"encoding/base32"
)

const (
	cidVersion  = 0x01
	codecRaw    = 0x55
	mhSHA256    = 0x12
	sha256Bytes = 32
)

var encoding = base32.StdEncoding.WithPadding(base32.NoPadding)

func Sum(data []byte) string {
	digest := sha256.Sum256(data)
	buf := make([]byte, 0, 2+2+sha256Bytes)
	buf = appendUvarint(buf, cidVersion)
	buf = appendUvarint(buf, codecRaw)
	buf = appendUvarint(buf, mhSHA256)
	buf = appendUvarint(buf, sha256Bytes)
	buf = append(buf, digest[:]...)
	return "b" + lower(encoding.EncodeToString(buf))
}

func appendUvarint(dst []byte, n uint64) []byte {
	for n >= 0x80 {
		dst = append(dst, byte(n)|0x80)
		n >>= 7
	}
	return append(dst, byte(n))
}

func lower(s string) string {
	out := []byte(s)
	for i := range out {
		if out[i] >= 'A' && out[i] <= 'Z' {
			out[i] += 'a' - 'A'
		}
	}
	return string(out)
}
