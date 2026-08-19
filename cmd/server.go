package cmd

import (
	"go-commerce/config"
	"go-commerce/rest"
	"log"
)

func Serve() {
	
	server := rest.NewServer(config.GetConfig())
	if err := server.Start(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
