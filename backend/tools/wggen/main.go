// Tool: wggen — generate WireGuard keypairs via wgcrypto for T1
// probing / manual testing. Lives under tools/ (separate from
// cmd/dashboard, the production binary) so `go build ./cmd/...`
// doesn't sweep it into release artifacts. Run with
// `go run ./tools/wggen` from the backend root.
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
