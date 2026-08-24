package main

import (
	"log"

	"go-commerce/cmd"
	"go-commerce/config"
)

func main() {
	// The one place the configuration is loaded. Everything below receives it,
	// which is why no package needs a package-level getter of its own.
	//
	// It is reported as an error rather than fatal inside config, so the decision
	// to end the process stays here.
	if _, err := config.LoadConfig(); err != nil {
		log.Fatalf("configuration failed to load: %v", err)
	}

	cmd.Serve()
}
