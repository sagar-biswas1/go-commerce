package main

import (
	"fmt"
	"os"

	"go-commerce/cmd"
)

const usage = `Usage: go-commerce <command> [arguments]

Commands:
  serve              Run the API (the default when no command is given)
  migrate <command>  Manage the database schema; try "migrate help"`

// main dispatches and does nothing else.
//
// Configuration is loaded by the command that needs it rather than here, so
// writing a migration works on a machine with no .env and no database -- the
// two things a new contributor is least likely to have set up first.
func main() {
	// No command means serve, so the everyday case stays `go run .`.
	command, args := "serve", []string{}
	if len(os.Args) > 1 {
		command, args = os.Args[1], os.Args[2:]
	}

	switch command {
	case "serve":
		cmd.Serve()
	case "migrate":
		// The exit code is decided by the subcommand and applied here, so this
		// stays the only place the process ends.
		os.Exit(cmd.Migrate(args))
	case "-h", "--help", "help":
		fmt.Println(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s\n", command, usage)
		os.Exit(2)
	}
}
