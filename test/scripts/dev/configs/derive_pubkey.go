//go:build ignore

// Tiny helper to print the validator public key for the well-known
// quickstart --local NODE_SEED. Run from the repo root via:
//   go run test/scripts/dev/configs/derive_pubkey.go
package main

import (
	"fmt"
	"os"

	"github.com/stellar/go-stellar-sdk/keypair"
)

func main() {
	seed := "SDQVDISRYN2JXBS7ICL7QJAEKB3HWBJFP2QECXG7GZICAHBK4UNJCWK2"
	if len(os.Args) > 1 {
		seed = os.Args[1]
	}
	kp, err := keypair.ParseFull(seed)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Seed:   %s\nPublic: %s\n", kp.Seed(), kp.Address())
}
