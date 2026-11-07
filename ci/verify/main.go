// Command verify checks an Argus release by hand.
//
// Put the downloaded files in one folder with SHA256SUMS and SHA256SUMS.sig, then:
//
//	go run ./ci/verify ~/Downloads/argus            # every listed file that's there
//	go run ./ci/verify ~/Downloads/argus/Argus-v0.15.1-linux-amd64.tar.gz
//
// -key overrides the public keys (base64 raw ed25519) built into the app.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/lacksonkapapahue/argus-desktop/internal/updsig"
)

func main() {
	key := flag.String("key", "", "base64 ed25519 public key (default: the keys built into the app)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: go run ./ci/verify [-key B64] DIR | FILE...")
		flag.PrintDefaults()
	}
	flag.Parse()
	pub := updsig.DefaultKeys()
	if *key != "" {
		k, err := updsig.ParsePublicKey(*key)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		pub = updsig.Keys{k}
	}
	args := flag.Args()
	if len(args) == 0 {
		args = []string{"."}
	}
	dir, files := args[0], args
	if st, err := os.Stat(args[0]); err == nil && st.IsDir() {
		files = nil
	} else {
		dir = filepath.Dir(args[0])
	}
	sums, _ := os.ReadFile(filepath.Join(dir, updsig.SumsName))
	sig, _ := os.ReadFile(filepath.Join(dir, updsig.SigName))
	if len(sums) == 0 {
		fmt.Fprintln(os.Stderr, "FAIL: no", updsig.SumsName, "in", dir)
		os.Exit(1)
	}
	if err := updsig.VerifySums(pub, sums, sig); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	fmt.Println("OK  signature on", updsig.SumsName)
	list, err := updsig.ParseSums(sums)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	if files == nil {
		names := make([]string, 0, len(list))
		for n := range list {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			if _, err := os.Stat(filepath.Join(dir, n)); err == nil {
				files = append(files, filepath.Join(dir, n))
			}
		}
	}
	bad := 0
	for _, f := range files {
		if err := updsig.VerifyFile(pub, sums, sig, filepath.Base(f), f); err != nil {
			fmt.Println("FAIL", f+":", err)
			bad++
		} else {
			fmt.Println("OK  ", f)
		}
	}
	if bad > 0 || len(files) == 0 {
		if len(files) == 0 {
			fmt.Fprintln(os.Stderr, "FAIL: none of the listed files are in", dir)
		}
		os.Exit(1)
	}
}
