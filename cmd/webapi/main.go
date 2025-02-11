package main

import (
	"log"
	"net/http"
	"time"

	"github.com/mikolajsemeniuk/blockchain/blockchain"
)

const port = ":8080"

func main() {
	chain, err := blockchain.NewChain()
	if err != nil {
		log.Fatal(err)
	}

	hdl := blockchain.NewHandler(chain)

	server := &http.Server{
		Addr:         port,
		Handler:      hdl,
		ReadTimeout:  time.Second * 5,
		WriteTimeout: time.Second * 15,
		IdleTimeout:  time.Second * 15,
	}

	log.Println("Listening on", port)
	log.Fatal(server.ListenAndServe())
}
