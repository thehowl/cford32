// Package cford32 implements a base32-like encoding/decoding package, with the
// encoding scheme [specified by Douglas Crockford].
//
// From the website, the requirements of said encoding scheme are to:
//
//   - Be human readable and machine readable.
//   - Be compact. Humans have difficulty in manipulating long strings of arbitrary symbols.
//   - Be error resistant. Entering the symbols must not require keyboarding gymnastics.
//   - Be pronounceable. Humans should be able to accurately transmit the symbols to other humans using a telephone.
//
// This is slightly different from a simple difference in encoding table from
// the Go's stdlib `encoding/base32`, as when decoding the characters i I l L are
// parsed as 1, and o O is parsed as 0.
//
// This package additionally provides ways to encode uint64's efficiently,
// as well as efficient encoding to a lowercase variation of the encoding.
// The encodings never use paddings.
//
// # Uint64 Encoding
//
// Aside from lower/uppercase encoding, there is a compact encoding, allowing
// to encode all values in [0,2^34), and the full encoding, allowing all
// values in [0,2^64). The compact encoding uses 7 characters, and the full
// encoding uses 13 characters. Both are parsed unambiguously by the Uint64
// decoder.
//
// The compact encodings have the first character between ['0','f'], while the
// full encoding's first character ranges between ['g','z']. Practically, in
// your usage of the package, you should consider which one to use and stick
// with it, while considering that the compact encoding, once it reaches 2^34,
// automatically switches to the full encoding. The properties of the generated
// strings are still maintained: for instance, any two encoded uint64s x,y
// consistently generated with the compact encoding, if the numeric value is
// x < y, will also be x < y in lexical ordering. However, values [0,2^34) have a
// "double encoding", which if mixed together lose the lexical ordering property.
//
// The Uint64 encoding is most useful for generating string versions of Uint64
// IDs. Practically, it allows you to retain sleek and compact IDs for your
// applcation for the first 2^34 (>17 billion) entities, while seamlessly
// rolling over to the full encoding should you exceed that. You are encouraged
// to use it unless you have a requirement or preferences for IDs consistently
// being always the same size.
//
// [specified by Douglas Crockford]: https://www.crockford.com/base32.html
package cford32

import (
	"io"
	"slices"
	"strconv"
)

const (
	encTable      = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	encTableLower = "0123456789abcdefghjkmnpqrstvwxyz"

	// each line is 16 bytes
	decTable = "" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" + // 00-0f
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" + // 10-1f
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" + // 20-2f
		"\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\xff\xff\xff\xff\xff\xff" + // 30-3f
		"\xff\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x01\x12\x13\x01\x14\x15\x00" + // 40-4f
		"\x16\x17\x18\x19\x1a\xff\x1b\x1c\x1d\x1e\x1f\xff\xff\xff\xff\xff" + // 50-5f
		"\xff\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x01\x12\x13\x01\x14\x15\x00" + // 60-6f
		"\x16\x17\x18\x19\x1a\xff\x1b\x1c\x1d\x1e\x1f\xff\xff\xff\xff\xff" + // 70-7f
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" + // 80-ff (not ASCII)
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff"
)

// CorruptInputError is returned by parsing functions when an invalid character
// in the input is found. The integer value represents the byte index where
// the error occurred.
//
// This is typically because the given character does not exist in the encoding.
type CorruptInputError int64

func (e CorruptInputError) Error() string {
	return "illegal cford32 data at input byte " + strconv.FormatInt(int64(e), 10)
}

// Uint64 parses a cford32-encoded byte slice into a uint64.
//
//   - The parser requires all provided character to be valid cford32 characters.
//   - The parser disregards case.
//   - If the first character is '0' <= c <= 'f', then the passed value is assumed
//     encoded in the compact encoding, and must be 7 characters long.
//   - If the first character is 'g' <= c <= 'z',  then the passed value is
//     assumed encoded in the full encoding, and must be 13 characters long.
//
// If any of these requirements fail, a CorruptInputError will be returned.
func Uint64(b []byte) (uint64, error) {
	switch {
	default:
		return 0, CorruptInputError(0)
	case len(b) == 7 && b[0] >= '0' && b[0] <= 'f':
		decVals := [7]byte{
			decTable[b[0]],
			decTable[b[1]],
			decTable[b[2]],
			decTable[b[3]],
			decTable[b[4]],
			decTable[b[5]],
			decTable[b[6]],
		}
		for idx, v := range decVals {
			if v >= 32 {
				return 0, CorruptInputError(idx)
			}
		}

		return 0 +
			uint64(decVals[0])<<30 |
			uint64(decVals[1])<<25 |
			uint64(decVals[2])<<20 |
			uint64(decVals[3])<<15 |
			uint64(decVals[4])<<10 |
			uint64(decVals[5])<<5 |
			uint64(decVals[6]), nil
	case len(b) == 13 && b[0] >= 'g' && b[0] <= 'z':
		decVals := [13]byte{
			decTable[b[0]] & 0x0F, // disregard high bit
			decTable[b[1]],
			decTable[b[2]],
			decTable[b[3]],
			decTable[b[4]],
			decTable[b[5]],
			decTable[b[6]],
			decTable[b[7]],
			decTable[b[8]],
			decTable[b[9]],
			decTable[b[10]],
			decTable[b[11]],
			decTable[b[12]],
		}
		for idx, v := range decVals {
			if v >= 32 {
				return 0, CorruptInputError(idx)
			}
		}

		return 0 +
			uint64(decVals[0])<<60 |
			uint64(decVals[1])<<55 |
			uint64(decVals[2])<<50 |
			uint64(decVals[3])<<45 |
			uint64(decVals[4])<<40 |
			uint64(decVals[5])<<35 |
			uint64(decVals[6])<<30 |
			uint64(decVals[7])<<25 |
			uint64(decVals[8])<<20 |
			uint64(decVals[9])<<15 |
			uint64(decVals[10])<<10 |
			uint64(decVals[11])<<5 |
			uint64(decVals[12]), nil
	}
}

const mask = 31

// PutUint64 returns a cford32-encoded byte slice.
func PutUint64(id uint64) [13]byte {
	return [13]byte{
		encTable[id>>60&mask|0x10], // specify full encoding
		encTable[id>>55&mask],
		encTable[id>>50&mask],
		encTable[id>>45&mask],
		encTable[id>>40&mask],
		encTable[id>>35&mask],
		encTable[id>>30&mask],
		encTable[id>>25&mask],
		encTable[id>>20&mask],
		encTable[id>>15&mask],
		encTable[id>>10&mask],
		encTable[id>>5&mask],
		encTable[id&mask],
	}
}

// PutUint64Lower returns a cford32-encoded byte array, swapping uppercase
// letters with lowercase.
//
// For more information on how the value is encoded, see [Uint64].
func PutUint64Lower(id uint64) [13]byte {
	return [13]byte{
		encTableLower[id>>60&mask|0x10],
		encTableLower[id>>55&mask],
		encTableLower[id>>50&mask],
		encTableLower[id>>45&mask],
		encTableLower[id>>40&mask],
		encTableLower[id>>35&mask],
		encTableLower[id>>30&mask],
		encTableLower[id>>25&mask],
		encTableLower[id>>20&mask],
		encTableLower[id>>15&mask],
		encTableLower[id>>10&mask],
		encTableLower[id>>5&mask],
		encTableLower[id&mask],
	}
}

// PutCompact returns a cford32-encoded byte slice, using the compact
// representation of cford32 described in the package documentation where
// possible (all values of id < 1<<34). The lowercase encoding is used.
//
// The resulting byte slice will be 7 bytes long for all compact values,
// and 13 bytes long for
func PutCompact(id uint64) []byte {
	return AppendCompact(id, nil)
}

// AppendCompact works like [PutCompact] but appends to the given byte slice
// instead of allocating one anew.
func AppendCompact(id uint64, b []byte) []byte {
	const maxCompact = 1 << 34
	if id < maxCompact {
		return append(b,
			encTableLower[id>>30&mask],
			encTableLower[id>>25&mask],
			encTableLower[id>>20&mask],
			encTableLower[id>>15&mask],
			encTableLower[id>>10&mask],
			encTableLower[id>>5&mask],
			encTableLower[id&mask],
		)
	}
	// XXX: does this allocate?
	res := PutUint64Lower(id)
	return append(b, res[:]...)
}

func DecodedLen(n int) int {
	return n * 5 / 8
}

func EncodedLen(n int) int {
	return (n*8 + 4) / 5
}

// Encode encodes src using the encoding enc,
// writing [EncodedLen](len(src)) bytes to dst.
//
// The encoding pads the output to a multiple of 8 bytes,
// so Encode is not appropriate for use on individual blocks
// of a large data stream. Use [NewEncoder] instead.
func Encode(dst, src []byte) {
	// Copied from encoding/base32/base32.go (go1.22)
	if len(src) == 0 {
		return
	}

	di, si := 0, 0
	n := (len(src) / 5) * 5
	for si < n {
		// Combining two 32 bit loads allows the same code to be used
		// for 32 and 64 bit platforms.
		hi := uint32(src[si+0])<<24 | uint32(src[si+1])<<16 | uint32(src[si+2])<<8 | uint32(src[si+3])
		lo := hi<<8 | uint32(src[si+4])

		dst[di+0] = encTable[(hi>>27)&0x1F]
		dst[di+1] = encTable[(hi>>22)&0x1F]
		dst[di+2] = encTable[(hi>>17)&0x1F]
		dst[di+3] = encTable[(hi>>12)&0x1F]
		dst[di+4] = encTable[(hi>>7)&0x1F]
		dst[di+5] = encTable[(hi>>2)&0x1F]
		dst[di+6] = encTable[(lo>>5)&0x1F]
		dst[di+7] = encTable[(lo)&0x1F]

		si += 5
		di += 8
	}

	// Add the remaining small block
	remain := len(src) - si
	if remain == 0 {
		return
	}

	// Encode the remaining bytes in reverse order.
	val := uint32(0)
	switch remain {
	case 4:
		val |= uint32(src[si+3])
		dst[di+6] = encTable[val<<3&0x1F]
		dst[di+5] = encTable[val>>2&0x1F]
		fallthrough
	case 3:
		val |= uint32(src[si+2]) << 8
		dst[di+4] = encTable[val>>7&0x1F]
		fallthrough
	case 2:
		val |= uint32(src[si+1]) << 16
		dst[di+3] = encTable[val>>12&0x1F]
		dst[di+2] = encTable[val>>17&0x1F]
		fallthrough
	case 1:
		val |= uint32(src[si+0]) << 24
		dst[di+1] = encTable[val>>22&0x1F]
		dst[di+0] = encTable[val>>27&0x1F]
	}
}

func EncodeLower(dst, src []byte) {
	// Copied from encoding/base32/base32.go (go1.22)
	if len(src) == 0 {
		return
	}

	di, si := 0, 0
	n := (len(src) / 5) * 5
	for si < n {
		// Combining two 32 bit loads allows the same code to be used
		// for 32 and 64 bit platforms.
		hi := uint32(src[si+0])<<24 | uint32(src[si+1])<<16 | uint32(src[si+2])<<8 | uint32(src[si+3])
		lo := hi<<8 | uint32(src[si+4])

		dst[di+0] = encTableLower[(hi>>27)&0x1F]
		dst[di+1] = encTableLower[(hi>>22)&0x1F]
		dst[di+2] = encTableLower[(hi>>17)&0x1F]
		dst[di+3] = encTableLower[(hi>>12)&0x1F]
		dst[di+4] = encTableLower[(hi>>7)&0x1F]
		dst[di+5] = encTableLower[(hi>>2)&0x1F]
		dst[di+6] = encTableLower[(lo>>5)&0x1F]
		dst[di+7] = encTableLower[(lo)&0x1F]

		si += 5
		di += 8
	}

	// Add the remaining small block
	remain := len(src) - si
	if remain == 0 {
		return
	}

	// Encode the remaining bytes in reverse order.
	val := uint32(0)
	switch remain {
	case 4:
		val |= uint32(src[si+3])
		dst[di+6] = encTableLower[val<<3&0x1F]
		dst[di+5] = encTableLower[val>>2&0x1F]
		fallthrough
	case 3:
		val |= uint32(src[si+2]) << 8
		dst[di+4] = encTableLower[val>>7&0x1F]
		fallthrough
	case 2:
		val |= uint32(src[si+1]) << 16
		dst[di+3] = encTableLower[val>>12&0x1F]
		dst[di+2] = encTableLower[val>>17&0x1F]
		fallthrough
	case 1:
		val |= uint32(src[si+0]) << 24
		dst[di+1] = encTableLower[val>>22&0x1F]
		dst[di+0] = encTableLower[val>>27&0x1F]
	}
}

// AppendEncode appends the cford32 encoded src to dst
// and returns the extended buffer.
func AppendEncode(dst, src []byte) []byte {
	n := EncodedLen(len(src))
	dst = slices.Grow(dst, n)
	Encode(dst[len(dst):][:n], src)
	return dst[:len(dst)+n]
}

// EncodeToString returns the cford32 encoding of src.
func EncodeToString(src []byte) string {
	buf := make([]byte, EncodedLen(len(src)))
	Encode(buf, src)
	return string(buf)
}

// EncodeToStringLower returns the cford32 lowercase encoding of src.
func EncodeToStringLower(src []byte) string {
	buf := make([]byte, EncodedLen(len(src)))
	EncodeLower(buf, src)
	return string(buf)
}

func Decode(dst, src []byte) (int, error) {
	panic("not implemented")
}

func DecodeString(s string) ([]byte, error) {
	panic("not implemented")
}

// Encoder/decoder functions
func NewDecoder(r io.Reader) io.Reader {
	panic("not implemented")
}

func NewEncoder(w io.Writer) io.Writer {
	panic("not implemented")
}

func NewEncoderLower(w io.Writer) io.Writer {
	panic("not implemented")
}
