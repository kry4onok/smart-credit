// Command keygen creates a persistent signing key without replacing existing keys.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/infra/jwt"
)

func main() {
	path := flag.String("out", ".data/auth/private.pem", "path for a new RSA private key")
	flag.Parse()
	if err := jwt.GenerateKey(*path); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("RSA signing key created:", *path)
}
