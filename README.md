# cford32
Package cford32 implements a base32-like encoding/decoding package, with the
encoding scheme [specified by Douglas Crockford].

[specified by Douglas Crockford]: https://www.crockford.com/base32.html

From the website, the requirements of said encoding scheme are to:

- Be human readable and machine readable.
- Be compact. Humans have difficulty in manipulating long strings of arbitrary symbols.
- Be error resistant. Entering the symbols must not require keyboarding gymnastics.
- Be pronounceable. Humans should be able to accurately transmit the symbols to other humans using a telephone.

This is slightly different from a simple difference in encoding table from
the Go's stdlib `encoding/base32`, as when decoding the characters i I l L are
parsed as 1, and o O is parsed as 0.

This package additionally provides ways to encode uint64's efficiently,
as well as efficient encoding to a lowercase variation of the encoding.
The encodings never use paddings.
