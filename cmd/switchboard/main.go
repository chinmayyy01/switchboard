package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/chinmay/switchboard/internal/ws"
)

func main() {
	addr := flag.String("addr", ":8090", "listen address")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", ws.Handler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	})

	log.Printf("switchboard realtime service on http://localhost%s", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
