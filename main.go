package main

import (
	"go-commerce/cmd"
	"go-commerce/config"
	"log"
)

func main() {
	// The one place the configuration is loaded. Everything below receives it.
	_, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Config failed to load: %v", err)
	}

	cmd.Serve()
}
