// Command fileserver is a tiny static HTTP file server for the bootstrap E2E
// scripts. It replaces `python3 -m http.server`, which on some macOS CI runners
// is absent or stalls on per-request reverse-DNS logging (a cause of flaky
// connect timeouts). It binds loopback-only and logs to stderr.
//
// Usage: fileserver <dir> <addr>   e.g. fileserver ./web 127.0.0.1:8799
package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatalf("usage: fileserver <dir> <addr>")
	}
	dir, addr := os.Args[1], os.Args[2]
	log.Printf("fileserver: serving %s on %s", dir, addr)
	if err := http.ListenAndServe(addr, http.FileServer(http.Dir(dir))); err != nil {
		log.Fatalf("fileserver: %v", err)
	}
}
