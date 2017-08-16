// Copyright (c) 2017 IBM Corp. Licensed Materials - Property of IBM.

package token

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	var ch Server
	var err error

	log.SetFlags(log.Ldate | log.Lmicroseconds)

	cfg, err := NewConfig()
	if err != nil {
		log.Println("Failed to create the Clear Harbor service: ", err)
	}

	ch, err = NewValidationServer(cfg)
	if err != nil {
		log.Println("Failed to create the Clear Harbor service: ", err)
		log.Println("Switching to deny-all mode")
		ch = denyallServer(cfg)
	}

	go func() {
		err := ch.Start()
		log.Fatal(err) // failed to start the server
	}()

	// Wait for a shutdown signal
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	// Shutdown the service and give it 30 seconds for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err = ch.Shutdown(ctx)
	if err != nil {
		log.Fatal("Failed to shutdown the Clear Harbor server: ", err)
	}
}

type denyall struct {
	http.Server
}

func denyallServer(cfg *Config) Server {
	spec := fmt.Sprintf(":%d", defaultPort)
	if cfg != nil {
		spec = cfg.HTTPAddressSpec
	}

	return &denyall{
		http.Server{
			Addr:         spec,
			ReadTimeout:  2 * time.Second, // reads from Ingress Proxy on localhost, so expect quick completion
			WriteTimeout: 2 * time.Second, // may need some more time to fetch expired keys from source
			Handler:      statusCodeHandler(http.StatusUnauthorized),
		},
	}

}

func (d *denyall) Start() error {
	log.Println("Starting the Clear Harbor service on ", d.Addr)

	go func() {
		log.Println("Clear Harbor configured to deny all")
		for range time.NewTicker(5 * time.Minute).C {
			log.Println("Clear Harbor configured to deny all")
		}
	}()

	return d.ListenAndServe()
}
