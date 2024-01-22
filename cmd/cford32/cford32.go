package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/thehowl/cford32"
)

const usageString = `Usage: %s [OPTION...] [FILE]
cford32 encode or decode FILE, or standard input, to standard output.
With no FILE, or when FILE is -, read standard input.

`

func main() {
	var (
		dec = flag.Bool("d", false, "decode data")
		lo  = flag.Bool("l", true, "use lowercase encoding")
		u64 = flag.Bool("n", false, "encode a uint64, or decode a cford32-encoded compact uint64")
	)
	_ = u64
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usageString, os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	f := os.Stdin
	var err error
	if arg := flag.Arg(0); arg != "" && arg != "-" {
		f, err = os.Open(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error opening %q: %v", arg, err)
			os.Exit(1)
		}
	}

	switch {
	case *u64 && *dec:
		buf, err := io.ReadAll(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading stdin: %v", err)
			os.Exit(1)
		}
		n, err := cford32.Uint64(bytes.TrimSpace(buf))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading uint64: %v", err)
			os.Exit(1)
		}
		fmt.Println(n)
	case *u64 && !*dec:
		buf, err := io.ReadAll(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading stdin: %v", err)
			os.Exit(1)
		}
		u, err := strconv.ParseUint(string(bytes.TrimSpace(buf)), 0, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error parsing integer: %v", err)
			os.Exit(1)
		}
		res := cford32.PutCompact(u)
		if !*lo {
			res = bytes.ToUpper(res)
		}
		fmt.Println(string(res))
	case !*u64 && *dec:
		dec := cford32.NewDecoder(f)
		_, err := io.Copy(os.Stdout, dec)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error decoding: %v", err)
			os.Exit(1)
		}
	case !*u64 && !*dec:
		var enc io.WriteCloser
		if *lo {
			enc = cford32.NewEncoderLower(os.Stdout)
		} else {
			enc = cford32.NewEncoder(os.Stdout)
		}
		_, err := io.Copy(enc, f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error encoding: %v", err)
			os.Exit(1)
		}
		if err := enc.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "error encoding: %v", err)
			os.Exit(1)
		}
		os.Stdout.Write([]byte("\n"))
	}
}
