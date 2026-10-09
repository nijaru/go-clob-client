// This example constructs a CLOB client from an EOA signer without exporting
// its private key. Replace the local adapter with a hardware/remote signer that
// implements signing.Signer; optional MessageSigner and TransactionSigner enable
// Safe/Proxy relaying and SDK-managed EOA transaction broadcasting respectively.
// Construction and this example perform no network requests or live mutations.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/nijaru/go-clob-client/clob"
	"github.com/nijaru/go-clob-client/signing"
)

func main() {
	signer, err := signing.NewLocalSigner(os.Getenv("PRIVATE_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	client, err := clob.NewSignerClient(clob.Config{Signer: signer})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(client.Address())
}
