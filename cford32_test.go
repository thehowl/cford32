package cford32

import (
	"bytes"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompactRoundtrip(t *testing.T) {
	buf := make([]byte, 13)
	prev := make([]byte, 13)
	for i := uint64(0); i < (1 << 15); i++ {
		res := AppendCompact(i, buf[:0])
		back, err := Uint64(res)
		assert.NoError(t, err)
		assert.Equal(t, back, i, "%q: mismatch between encoded value (%d) and retrieved value (%d)", string(buf), i, back)

		assert.Equal(t, -1, bytes.Compare(prev, res), "lexicographic order test")
		prev, buf = res, prev
	}
	for i := uint64(1<<34 - 1024); i < (1<<34 + 1024); i++ {
		res := AppendCompact(i, buf[:0])
		back, err := Uint64(res)
		// println(string(res))
		assert.NoError(t, err)
		assert.Equal(t, back, i, "%q: mismatch between encoded value (%d) and retrieved value (%d)", string(buf), i, back)

		assert.Equal(t, -1, bytes.Compare(prev, res), "lexicographic order test")
		prev, buf = res, prev
	}
	for i := uint64(1<<64 - 5000); i != 0; i++ {
		res := AppendCompact(i, buf[:0])
		back, err := Uint64(res)
		assert.NoError(t, err)
		assert.Equal(t, back, i, "%q: mismatch between encoded value (%d) and retrieved value (%d)", string(buf), i, back)

		assert.Equal(t, -1, bytes.Compare(prev, res), "lexicographic order test")
		prev, buf = res, prev
	}
}

func BenchmarkCompact(b *testing.B) {
	buf := make([]byte, 13)
	for i := 0; i < b.N; i++ {
		_ = AppendCompact(uint64(i), buf[:0])
	}
}

type testpair struct {
	decoded, encoded string
}

var pairs = []testpair{
	{"", ""},
	{"f", "CR"},
	{"fo", "CSQG"},
	{"foo", "CSQPY"},
	{"foob", "CSQPYRG"},
	{"fooba", "CSQPYRK1"},
	{"foobar", "CSQPYRK1E8"},

	{"sure.", "EDTQ4S9E"},
	{"sure", "EDTQ4S8"},
	{"sur", "EDTQ4"},
	{"su", "EDTG"},
	{"leasure.", "DHJP2WVNE9JJW"},
	{"easure.", "CNGQ6XBJCMQ0"},
	{"asure.", "C5SQAWK55R"},
}

var bigtest = testpair{
	"Twas brillig, and the slithy toves",
	"AHVP2WS0C9S6JV3CD5KJR831DSJ20X38CMG76V39EHM7J83MDXV6AWR",
}

func testEqual(t *testing.T, msg string, args ...any) bool {
	t.Helper()
	if args[len(args)-2] != args[len(args)-1] {
		t.Errorf(msg, args...)
		return false
	}
	return true
}

func TestEncode(t *testing.T) {
	for _, p := range pairs {
		got := EncodeToString([]byte(p.decoded))
		testEqual(t, "Encode(%q) = %q, want %q", p.decoded, got, p.encoded)
		dst := AppendEncode([]byte("lead"), []byte(p.decoded))
		testEqual(t, `AppendEncode("lead", %q) = %q, want %q`, p.decoded, string(dst), "lead"+p.encoded)
	}
}

func TestEncoder(t *testing.T) {
	for _, p := range pairs {
		bb := &strings.Builder{}
		encoder := NewEncoder(bb)
		encoder.Write([]byte(p.decoded))
		encoder.Close()
		testEqual(t, "Encode(%q) = %q, want %q", p.decoded, bb.String(), p.encoded)
	}
}

func TestEncoderBuffering(t *testing.T) {
	input := []byte(bigtest.decoded)
	for bs := 1; bs <= 12; bs++ {
		bb := &strings.Builder{}
		encoder := NewEncoder(bb)
		for pos := 0; pos < len(input); pos += bs {
			end := pos + bs
			if end > len(input) {
				end = len(input)
			}
			n, err := encoder.Write(input[pos:end])
			testEqual(t, "Write(%q) gave error %v, want %v", input[pos:end], err, error(nil))
			testEqual(t, "Write(%q) gave length %v, want %v", input[pos:end], n, end-pos)
		}
		err := encoder.Close()
		testEqual(t, "Close gave error %v, want %v", err, error(nil))
		testEqual(t, "Encoding/%d of %q = %q, want %q", bs, bigtest.decoded, bb.String(), bigtest.encoded)
	}
}

func TestDecode(t *testing.T) {
	for _, p := range pairs {
		dbuf := make([]byte, DecodedLen(len(p.encoded)))
		count, _, err := decode(dbuf, []byte(p.encoded))
		testEqual(t, "Decode(%q) = error %v, want %v", p.encoded, err, error(nil))
		testEqual(t, "Decode(%q) = length %v, want %v", p.encoded, count, len(p.decoded))
		testEqual(t, "Decode(%q) = %q, want %q", p.encoded, string(dbuf[0:count]), p.decoded)

		dbuf, err = DecodeString(p.encoded)
		testEqual(t, "DecodeString(%q) = error %v, want %v", p.encoded, err, error(nil))
		testEqual(t, "DecodeString(%q) = %q, want %q", p.encoded, string(dbuf), p.decoded)

		dst, err := AppendDecode([]byte("lead"), []byte(p.encoded))
		testEqual(t, "AppendDecode(%q) = error %v, want %v", p.encoded, err, error(nil))
		testEqual(t, `AppendDecode("lead", %q) = %q, want %q`, p.encoded, string(dst), "lead"+p.decoded)

		dst2, err := AppendDecode(dst[:0:len(p.decoded)], []byte(p.encoded))
		testEqual(t, "AppendDecode(%q) = error %v, want %v", p.encoded, err, error(nil))
		testEqual(t, `AppendDecode("", %q) = %q, want %q`, p.encoded, string(dst2), p.decoded)
		if len(dst) > 0 && len(dst2) > 0 && &dst[0] != &dst2[0] {
			t.Errorf("unexpected capacity growth: got %d, want %d", cap(dst2), cap(dst))
		}
	}
}

func TestDecoder(t *testing.T) {
	for _, p := range pairs {
		decoder := NewDecoder(strings.NewReader(p.encoded))
		dbuf := make([]byte, DecodedLen(len(p.encoded)))
		count, err := decoder.Read(dbuf)
		if err != nil && err != io.EOF {
			t.Fatal("Read failed", err)
		}
		testEqual(t, "Read from %q = length %v, want %v", p.encoded, count, len(p.decoded))
		testEqual(t, "Decoding of %q = %q, want %q", p.encoded, string(dbuf[0:count]), p.decoded)
		if err != io.EOF {
			_, err = decoder.Read(dbuf)
		}
		testEqual(t, "Read from %q = %v, want %v", p.encoded, err, io.EOF)
	}
}

type badReader struct {
	data   []byte
	errs   []error
	called int
	limit  int
}

// Populates p with data, returns a count of the bytes written and an
// error.  The error returned is taken from badReader.errs, with each
// invocation of Read returning the next error in this slice, or io.EOF,
// if all errors from the slice have already been returned.  The
// number of bytes returned is determined by the size of the input buffer
// the test passes to decoder.Read and will be a multiple of 8, unless
// badReader.limit is non zero.
func (b *badReader) Read(p []byte) (int, error) {
	lim := len(p)
	if b.limit != 0 && b.limit < lim {
		lim = b.limit
	}
	if len(b.data) < lim {
		lim = len(b.data)
	}
	for i := range p[:lim] {
		p[i] = b.data[i]
	}
	b.data = b.data[lim:]
	err := io.EOF
	if b.called < len(b.errs) {
		err = b.errs[b.called]
	}
	b.called++
	return lim, err
}

// TestIssue20044 tests that decoder.Read behaves correctly when the caller
// supplied reader returns an error.
func TestIssue20044(t *testing.T) {
	badErr := errors.New("bad reader error")
	testCases := []struct {
		r       badReader
		res     string
		err     error
		dbuflen int
	}{
		// Check valid input data accompanied by an error is processed and the error is propagated.
		{
			r:   badReader{data: []byte("cr"), errs: []error{badErr}},
			res: "f", err: badErr,
		},
		// Check a read error accompanied by input data consisting of newlines only is propagated.
		{
			r:   badReader{data: []byte("\n\n\n\n\n\n\n\n"), errs: []error{badErr, nil}},
			res: "", err: badErr,
		},
		// Reader will be called twice.  The first time it will return 8 newline characters.  The
		// second time valid base32 encoded data and an error.  The data should be decoded
		// correctly and the error should be propagated.
		{
			r:   badReader{data: []byte("\n\n\n\n\n\n\n\ncr"), errs: []error{nil, badErr}},
			res: "f", err: badErr, dbuflen: 8,
		},
		// Reader returns invalid input data (too short) and an error.  Verify the reader
		// error is returned.
		{
			r:   badReader{data: []byte("c"), errs: []error{badErr}},
			res: "", err: badErr,
		},
		// Reader returns invalid input data (too short) but no error.  Verify io.ErrUnexpectedEOF
		// is returned.
		// NOTE(thehowl): I don't think this should applyto us?
		/* {
			r:   badReader{data: []byte("c"), errs: []error{nil}},
			res: "", err: io.ErrUnexpectedEOF,
		},*/
		// Reader returns invalid input data and an error.  Verify the reader and not the
		// decoder error is returned.
		{
			r:   badReader{data: []byte("cu"), errs: []error{badErr}},
			res: "", err: badErr,
		},
		// Reader returns valid data and io.EOF.  Check data is decoded and io.EOF is propagated.
		{
			r:   badReader{data: []byte("csqpyrk1"), errs: []error{io.EOF}},
			res: "fooba", err: io.EOF,
		},
		// Check errors are properly reported when decoder.Read is called multiple times.
		// decoder.Read will be called 8 times, badReader.Read will be called twice, returning
		// valid data both times but an error on the second call.
		{
			r:   badReader{data: []byte("dhjp2wvne9jjw"), errs: []error{nil, badErr}},
			res: "leasure.", err: badErr, dbuflen: 1,
		},
		// Check io.EOF is properly reported when decoder.Read is called multiple times.
		// decoder.Read will be called 8 times, badReader.Read will be called twice, returning
		// valid data both times but io.EOF on the second call.
		{
			r:   badReader{data: []byte("dhjp2wvne9jjw"), errs: []error{nil, io.EOF}},
			res: "leasure.", err: io.EOF, dbuflen: 1,
		},
		// The following two test cases check that errors are propagated correctly when more than
		// 8 bytes are read at a time.
		{
			r:   badReader{data: []byte("dhjp2wvne9jjw"), errs: []error{io.EOF}},
			res: "leasure.", err: io.EOF, dbuflen: 11,
		},
		{
			r:   badReader{data: []byte("dhjp2wvne9jjw"), errs: []error{badErr}},
			res: "leasure.", err: badErr, dbuflen: 11,
		},
		// Check that errors are correctly propagated when the reader returns valid bytes in
		// groups that are not divisible by 8.  The first read will return 11 bytes and no
		// error.  The second will return 7 and an error.  The data should be decoded correctly
		// and the error should be propagated.
		// NOTE(thehowl): again, this is on the assumption that this is padded, and it's not.
		/* {
			r:   badReader{data: []byte("dhjp2wvne9jjw"), errs: []error{nil, badErr}, limit: 11},
			res: "leasure.", err: badErr,
		}, */
	}

	for _, tc := range testCases {
		input := tc.r.data
		decoder := NewDecoder(&tc.r)
		var dbuflen int
		if tc.dbuflen > 0 {
			dbuflen = tc.dbuflen
		} else {
			dbuflen = DecodedLen(len(input))
		}
		dbuf := make([]byte, dbuflen)
		var err error
		var res []byte
		for err == nil {
			var n int
			n, err = decoder.Read(dbuf)
			if n > 0 {
				res = append(res, dbuf[:n]...)
			}
		}

		testEqual(t, "Decoding of %q = %q, want %q", string(input), string(res), tc.res)
		testEqual(t, "Decoding of %q err = %v, expected %v", string(input), err, tc.err)
	}
}

// TestDecoderError verifies decode errors are propagated when there are no read
// errors.
func TestDecoderError(t *testing.T) {
	for _, readErr := range []error{io.EOF, nil} {
		input := "csqpyrk1"
		dbuf := make([]byte, DecodedLen(len(input)))
		br := badReader{data: []byte(input), errs: []error{readErr}}
		decoder := NewDecoder(&br)
		n, err := decoder.Read(dbuf)
		testEqual(t, "Read after EOF, n = %d, expected %d", n, 0)
		if _, ok := err.(CorruptInputError); !ok {
			t.Errorf("Corrupt input error expected.  Found %T", err)
		}
	}
}

// TestReaderEOF ensures decoder.Read behaves correctly when input data is
// exhausted.
func TestReaderEOF(t *testing.T) {
	for _, readErr := range []error{io.EOF, nil} {
		input := "MZXW6YTB"
		br := badReader{data: []byte(input), errs: []error{nil, readErr}}
		decoder := NewDecoder(&br)
		dbuf := make([]byte, DecodedLen(len(input)))
		n, err := decoder.Read(dbuf)
		testEqual(t, "Decoding of %q err = %v, expected %v", input, err, error(nil))
		n, err = decoder.Read(dbuf)
		testEqual(t, "Read after EOF, n = %d, expected %d", n, 0)
		testEqual(t, "Read after EOF, err = %v, expected %v", err, io.EOF)
		n, err = decoder.Read(dbuf)
		testEqual(t, "Read after EOF, n = %d, expected %d", n, 0)
		testEqual(t, "Read after EOF, err = %v, expected %v", err, io.EOF)
	}
}

func TestDecoderBuffering(t *testing.T) {
	for bs := 1; bs <= 12; bs++ {
		decoder := NewDecoder(strings.NewReader(bigtest.encoded))
		buf := make([]byte, len(bigtest.decoded)+12)
		var total int
		var n int
		var err error
		for total = 0; total < len(bigtest.decoded) && err == nil; {
			n, err = decoder.Read(buf[total : total+bs])
			total += n
		}
		if err != nil && err != io.EOF {
			t.Errorf("Read from %q at pos %d = %d, unexpected error %v", bigtest.encoded, total, n, err)
		}
		testEqual(t, "Decoding/%d of %q = %q, want %q", bs, bigtest.encoded, string(buf[0:total]), bigtest.decoded)
	}
}

func TestDecodeCorrupt(t *testing.T) {
	testCases := []struct {
		input  string
		offset int // -1 means no corruption.
	}{
		{"", -1},
		{"!!!!", 0},
		{"x===", 0},
		{"AA=A====", 2},
		{"AAA=AAAA", 3},
		{"MMMMMMMMM", 8},
		{"MMMMMM", 0},
		{"A=", 1},
		{"AA=", 3},
		{"AA==", 4},
		{"AA===", 5},
		{"AAAA=", 5},
		{"AAAA==", 6},
		{"AAAAA=", 6},
		{"AAAAA==", 7},
		{"A=======", 1},
		{"AA======", -1},
		{"AAA=====", 3},
		{"AAAA====", -1},
		{"AAAAA===", -1},
		{"AAAAAA==", 6},
		{"AAAAAAA=", -1},
		{"AAAAAAAA", -1},
	}
	for _, tc := range testCases {
		dbuf := make([]byte, DecodedLen(len(tc.input)))
		_, err := Decode(dbuf, []byte(tc.input))
		if tc.offset == -1 {
			if err != nil {
				t.Error("Decoder wrongly detected corruption in", tc.input)
			}
			continue
		}
		switch err := err.(type) {
		case CorruptInputError:
			testEqual(t, "Corruption in %q at offset %v, want %v", tc.input, int(err), tc.offset)
		default:
			t.Error("Decoder failed to detect corruption in", tc)
		}
	}
}

func TestBig(t *testing.T) {
	n := 3*1000 + 1
	raw := make([]byte, n)
	const alpha = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	for i := 0; i < n; i++ {
		raw[i] = alpha[i%len(alpha)]
	}
	encoded := new(bytes.Buffer)
	w := NewEncoder(encoded)
	nn, err := w.Write(raw)
	if nn != n || err != nil {
		t.Fatalf("Encoder.Write(raw) = %d, %v want %d, nil", nn, err, n)
	}
	err = w.Close()
	if err != nil {
		t.Fatalf("Encoder.Close() = %v want nil", err)
	}
	decoded, err := io.ReadAll(NewDecoder(encoded))
	if err != nil {
		t.Fatalf("io.ReadAll(NewDecoder(...)): %v", err)
	}

	if !bytes.Equal(raw, decoded) {
		var i int
		for i = 0; i < len(decoded) && i < len(raw); i++ {
			if decoded[i] != raw[i] {
				break
			}
		}
		t.Errorf("Decode(Encode(%d-byte string)) failed at offset %d", n, i)
	}
}

func testStringEncoding(t *testing.T, expected string, examples []string) {
	for _, e := range examples {
		buf, err := DecodeString(e)
		if err != nil {
			t.Errorf("Decode(%q) failed: %v", e, err)
			continue
		}
		if s := string(buf); s != expected {
			t.Errorf("Decode(%q) = %q, want %q", e, s, expected)
		}
	}
}

func TestNewLineCharacters(t *testing.T) {
	// Each of these should decode to the string "sure", without errors.
	examples := []string{
		"EDTQ4S8",
		"EDTQ4S8\r",
		"EDTQ4S8\n",
		"EDTQ4S8\r\n",
		"EDTQ4S\r\n8",
		"EDT\rQ4S\n8",
		"edt\nq4s\r8",
		"edt\nq4s8",
		"EDTQ4S\n8",
	}
	testStringEncoding(t, "sure", examples)
}

func BenchmarkEncode(b *testing.B) {
	data := make([]byte, 8192)
	buf := make([]byte, EncodedLen(len(data)))
	b.SetBytes(int64(len(data)))
	for i := 0; i < b.N; i++ {
		Encode(buf, data)
	}
}

func BenchmarkEncodeToString(b *testing.B) {
	data := make([]byte, 8192)
	b.SetBytes(int64(len(data)))
	for i := 0; i < b.N; i++ {
		EncodeToString(data)
	}
}

func BenchmarkDecode(b *testing.B) {
	data := make([]byte, EncodedLen(8192))
	Encode(data, make([]byte, 8192))
	buf := make([]byte, 8192)
	b.SetBytes(int64(len(data)))
	for i := 0; i < b.N; i++ {
		Decode(buf, data)
	}
}

func BenchmarkDecodeString(b *testing.B) {
	data := EncodeToString(make([]byte, 8192))
	b.SetBytes(int64(len(data)))
	for i := 0; i < b.N; i++ {
		DecodeString(data)
	}
}

func TestBufferedDecodingSameError(t *testing.T) {
	testcases := []struct {
		prefix            string
		chunkCombinations [][]string
		expected          error
	}{
		// NBSWY3DPO5XXE3DE == helloworld
		// Test with "ZZ" as extra input
		{"helloworld", [][]string{
			{"NBSW", "Y3DP", "O5XX", "E3DE", "ZZ"},
			{"NBSWY3DPO5XXE3DE", "ZZ"},
			{"NBSWY3DPO5XXE3DEZZ"},
			{"NBS", "WY3", "DPO", "5XX", "E3D", "EZZ"},
			{"NBSWY3DPO5XXE3", "DEZZ"},
		}, io.ErrUnexpectedEOF},

		// Test with "ZZY" as extra input
		{"helloworld", [][]string{
			{"NBSW", "Y3DP", "O5XX", "E3DE", "ZZY"},
			{"NBSWY3DPO5XXE3DE", "ZZY"},
			{"NBSWY3DPO5XXE3DEZZY"},
			{"NBS", "WY3", "DPO", "5XX", "E3D", "EZZY"},
			{"NBSWY3DPO5XXE3", "DEZZY"},
		}, io.ErrUnexpectedEOF},

		// Normal case, this is valid input
		{"helloworld", [][]string{
			{"NBSW", "Y3DP", "O5XX", "E3DE"},
			{"NBSWY3DPO5XXE3DE"},
			{"NBS", "WY3", "DPO", "5XX", "E3D", "E"},
			{"NBSWY3DPO5XXE3", "DE"},
		}, nil},

		// MZXW6YTB = fooba
		{"fooba", [][]string{
			{"MZXW6YTBZZ"},
			{"MZXW6YTBZ", "Z"},
			{"MZXW6YTB", "ZZ"},
			{"MZXW6YT", "BZZ"},
			{"MZXW6Y", "TBZZ"},
			{"MZXW6Y", "TB", "ZZ"},
			{"MZXW6", "YTBZZ"},
			{"MZXW6", "YTB", "ZZ"},
			{"MZXW6", "YT", "BZZ"},
		}, io.ErrUnexpectedEOF},

		// Normal case, this is valid input
		{"fooba", [][]string{
			{"MZXW6YTB"},
			{"MZXW6YT", "B"},
			{"MZXW6Y", "TB"},
			{"MZXW6", "YTB"},
			{"MZXW6", "YT", "B"},
			{"MZXW", "6YTB"},
			{"MZXW", "6Y", "TB"},
		}, nil},
	}

	for _, testcase := range testcases {
		for _, chunks := range testcase.chunkCombinations {
			pr, pw := io.Pipe()

			// Write the encoded chunks into the pipe
			go func() {
				for _, chunk := range chunks {
					pw.Write([]byte(chunk))
				}
				pw.Close()
			}()

			decoder := NewDecoder(pr)
			_, err := io.ReadAll(decoder)

			if err != testcase.expected {
				t.Errorf("Expected %v, got %v; case %s %+v", testcase.expected, err, testcase.prefix, chunks)
			}
		}
	}
}

func TestBufferedDecodingPadding(t *testing.T) {
	testcases := []struct {
		chunks        []string
		expectedError string
	}{
		{[]string{
			"I4======",
			"==",
		}, "unexpected EOF"},

		{[]string{
			"I4======N4======",
		}, "illegal base32 data at input byte 2"},

		{[]string{
			"I4======",
			"N4======",
		}, "illegal base32 data at input byte 0"},

		{[]string{
			"I4======",
			"========",
		}, "illegal base32 data at input byte 0"},

		{[]string{
			"I4I4I4I4",
			"I4======",
			"I4======",
		}, "illegal base32 data at input byte 0"},
	}

	for _, testcase := range testcases {
		testcase := testcase
		pr, pw := io.Pipe()
		go func() {
			for _, chunk := range testcase.chunks {
				_, _ = pw.Write([]byte(chunk))
			}
			_ = pw.Close()
		}()

		decoder := NewDecoder(pr)
		_, err := io.ReadAll(decoder)

		if err == nil && len(testcase.expectedError) != 0 {
			t.Errorf("case %q: got nil error, want %v", testcase.chunks, testcase.expectedError)
		} else if err.Error() != testcase.expectedError {
			t.Errorf("case %q: got %v, want %v", testcase.chunks, err, testcase.expectedError)
		}
	}
}

func TestEncodedLen(t *testing.T) {
	type test struct {
		n    int
		want int64
	}
	tests := []test{
		{0, 0},
		{1, 2},
		{2, 4},
		{3, 5},
		{4, 7},
		{5, 8},
		{6, 10},
		{7, 12},
		{10, 16},
		{11, 18},
	}
	// check overflow
	switch strconv.IntSize {
	case 32:
		tests = append(tests, test{(math.MaxInt-4)/8 + 1, 429496730})
		tests = append(tests, test{math.MaxInt/8*5 + 4, math.MaxInt})
	case 64:
		tests = append(tests, test{(math.MaxInt-4)/8 + 1, 1844674407370955162})
		tests = append(tests, test{math.MaxInt/8*5 + 4, math.MaxInt})
	}
	for _, tt := range tests {
		if got := EncodedLen(tt.n); int64(got) != tt.want {
			t.Errorf("EncodedLen(%d): got %d, want %d", tt.n, got, tt.want)
		}
	}
}

func TestDecodedLen(t *testing.T) {
	type test struct {
		n    int
		want int64
	}
	tests := []test{
		{0, 0},
		{2, 1},
		{4, 2},
		{5, 3},
		{7, 4},
		{8, 5},
		{10, 6},
		{12, 7},
		{16, 10},
		{18, 11},
	}
	// check overflow
	switch strconv.IntSize {
	case 32:
		tests = append(tests, test{math.MaxInt/5 + 1, 268435456})
		tests = append(tests, test{math.MaxInt, 1342177279})
	case 64:
		tests = append(tests, test{math.MaxInt/5 + 1, 1152921504606846976})
		tests = append(tests, test{math.MaxInt, 5764607523034234879})
	}
	for _, tt := range tests {
		if got := DecodedLen(tt.n); int64(got) != tt.want {
			t.Errorf("DecodedLen(%d): got %d, want %d", tt.n, got, tt.want)
		}
	}
}
