// Command sign writes SHA256SUMS.sig next to a SHA256SUMS file.
//
//	BLAZE_UPDATE_SIGNING_KEY=<base64 seed> go run ./ci/sign rel/SHA256SUMS
//
// The key is base64 of the 32-byte ed25519 seed (64-byte seed+public also works).
// It is read only from the environment and never printed. The tool refuses to sign
// with a key whose public half isn't one the app trusts (updsig.TrustedKeys), so a
// wrong secret can't publish releases that every app would reject.
package main

import (
	"crypto/ed25519"
	"fmt"
	"os"

	"github.com/lacksonkapapahue/argus-desktop/internal/updsig"
)

func die(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "sign: "+format+"\n", a...)
	os.Exit(1)
}

func main() {
	if len(os.Args) != 2 {
		die("usage: BLAZE_UPDATE_SIGNING_KEY=... go run ./ci/sign path/to/SHA256SUMS")
	}
	raw := os.Getenv("BLAZE_UPDATE_SIGNING_KEY")
	if raw == "" {
		die("BLAZE_UPDATE_SIGNING_KEY is not set")
	}
	priv, err := updsig.ParsePrivateKey(raw)
	if err != nil {
		die("%v", err) // describes the shape only, never the value
	}
	if !updsig.DefaultKeys().Has(priv.Public().(ed25519.PublicKey)) {
		die("BLAZE_UPDATE_SIGNING_KEY doesn't match any public key in updsig.TrustedKeys")
	}
	sums, err := os.ReadFile(os.Args[1])
	if err != nil {
		die("%v", err)
	}
	if _, err := updsig.ParseSums(sums); err != nil {
		die("%v", err)
	}
	sig := updsig.Sign(priv, sums)
	if err := updsig.VerifySums(updsig.DefaultKeys(), sums, sig); err != nil {
		die("self-check failed: %v", err)
	}
	if err := os.WriteFile(os.Args[1]+".sig", sig, 0o644); err != nil {
		die("%v", err)
	}
	fmt.Println("signed", os.Args[1])
}
