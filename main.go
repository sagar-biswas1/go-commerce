package main

import (
	"go-commerce/cmd"
	"go-commerce/config"
)

func main() {
	config.LoadConfig()
	cmd.Serve()
}
