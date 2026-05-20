// Tool: wggen — generate WireGuard keypairs via wgcrypto for T1 probing.
// Not shipped in the production binary; lives under cmd/ purely for
// `go run` access to the internal/ packages.
package main

import (
	"fmt"

	"github.com/cern/3xui-dashboard/internal/service/wgcrypto"
)

func main() {
	server, _ := wgcrypto.GenerateKeypair()
	peer1, _ := wgcrypto.GenerateKeypair()
	fmt.Printf("server_priv=%s\n", server.Private)
	fmt.Printf("server_pub=%s\n", server.Public)
	fmt.Printf("peer1_priv=%s\n", peer1.Private)
	fmt.Printf("peer1_pub=%s\n", peer1.Public)
}
