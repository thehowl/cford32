package cford32_test

import (
	"fmt"
	"math/rand"

	"github.com/thehowl/cford32"
)

func ExamplePutCompact() {
	gen := rand.New(rand.NewSource(12345))

	for i := 0; i < 6; i++ {
		// 1 << 35 allows us to show diverse set of compact/extended
		n := uint64(gen.Int63n(1 << 35))
		fmt.Printf("%11d: %s\n", n, string(cford32.PutCompact(n)))
	}

	// Output:
	// 14334683418: db6kt8t
	// 34093059390: g00000zr1nk9y
	//   417819965: 0ceev9x
	// 17538543416: g00000gap1vsr
	//  5407252823: 514r8aq
	// 16008560262: ex2yfm6
}

func ExampleUint64() {
	val, _ := cford32.Uint64([]byte("ex2yfm6"))
	fmt.Println(val)

	// Output:
	// 16008560262
}
