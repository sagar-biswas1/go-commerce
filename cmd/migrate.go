package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"go-commerce/config"
	"go-commerce/infra/db"
	"go-commerce/migration"
)

// migrateUsage is printed for an unknown subcommand and for -h.
const migrateUsage = `Usage: go-commerce migrate <command> [arguments]

Commands:
  generate <name>   Write a new empty up/down pair into ./migration
  up [n]            Apply pending migrations (all, or the next n)
  down [n]          Roll back the most recent migrations (default 1)
  redo              Roll back the last migration and apply it again
  status            Show every migration and whether it has run
  version           Print the highest applied version

Aliases: push = up, rollback = down, new = generate`

// Migrate is the migration CLI.
//
// It returns an exit code rather than calling os.Exit, so main stays the one
// place the process can end and a test could drive this without taking the
// test binary down with it.
func Migrate(args []string) int {
	if len(args) == 0 {
		fmt.Println(migrateUsage)
		return 2
	}

	command, rest := args[0], args[1:]

	// generate touches no database, which is the point: a new migration can be
	// written on a laptop with nothing running.
	switch command {
	case "generate", "new", "create":
		return generateMigration(rest)
	case "-h", "--help", "help":
		fmt.Println(migrateUsage)
		return 0
	}

	// Only the database-touching commands need configuration, which is why it is
	// loaded here rather than in main.
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration failed to load: %v\n", err)
		return 1
	}

	dbCon, err := db.GetConnection(cfg.PG)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not connect to the database: %v\n", err)
		return 1
	}
	defer dbCon.Close()

	runner := db.NewRunner(dbCon, migration.FS)
	ctx := context.Background()

	switch command {
	case "up", "push", "apply", "migrate":
		return migrateUp(ctx, runner, rest)
	case "down", "rollback":
		return migrateDown(ctx, runner, rest)
	case "redo":
		return migrateRedo(ctx, runner)
	case "status":
		return migrateStatus(ctx, runner)
	case "version":
		return migrateVersion(ctx, runner)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s\n", command, migrateUsage)
		return 2
	}
}

func generateMigration(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, `migrate generate needs a name, such as: migrate generate "add orders table"`)
		return 2
	}

	// Everything after the command is the name, so quoting it is optional.
	name := args[0]
	for _, extra := range args[1:] {
		name += " " + extra
	}

	upPath, downPath, err := db.Generate(migration.Dir, name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	fmt.Printf("created %s\ncreated %s\n\n", upPath, downPath)
	// The files are embedded at build time, so a migration written now is not
	// in the binary running now. Saying so here saves the confusion of a
	// generate followed by a status that does not list it.
	fmt.Println("Write the statements, then rebuild before running them: the .sql files are embedded.")
	return 0
}

func migrateUp(ctx context.Context, runner *db.Runner, args []string) int {
	limit, ok := optionalCount(args, 0)
	if !ok {
		return 2
	}

	ran, err := runner.Up(ctx, limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate up: %v\n", err)
		return 1
	}
	if len(ran) == 0 {
		fmt.Println("schema up to date; nothing to apply")
		return 0
	}

	fmt.Printf("applied %d migration(s)\n", len(ran))
	return 0
}

func migrateDown(ctx context.Context, runner *db.Runner, args []string) int {
	count, ok := optionalCount(args, 1)
	if !ok {
		return 2
	}

	rolled, err := runner.Down(ctx, count)
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate down: %v\n", err)
		return 1
	}
	if len(rolled) == 0 {
		fmt.Println("nothing to roll back")
		return 0
	}

	fmt.Printf("rolled back %d migration(s)\n", len(rolled))
	return 0
}

func migrateRedo(ctx context.Context, runner *db.Runner) int {
	rolled, err := runner.Down(ctx, 1)
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate redo: %v\n", err)
		return 1
	}
	if len(rolled) == 0 {
		fmt.Println("nothing to redo")
		return 0
	}

	if _, err := runner.Up(ctx, 1); err != nil {
		// The rollback already happened, so say what state this left behind
		// rather than only what failed.
		fmt.Fprintf(os.Stderr,
			"migrate redo: %s was rolled back but could not be re-applied: %v\n", rolled[0], err)
		return 1
	}

	fmt.Printf("redid %s\n", rolled[0])
	return 0
}

func migrateStatus(ctx context.Context, runner *db.Runner) int {
	report, err := runner.Status(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate status: %v\n", err)
		return 1
	}
	if len(report) == 0 {
		fmt.Println("no migrations")
		return 0
	}

	out := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(out, "VERSION\tNAME\tSTATE\tAPPLIED AT\tDOWN")

	pending, modified := 0, 0
	for _, s := range report {
		appliedAt := "-"
		if s.Applied != nil {
			appliedAt = s.Applied.AppliedAt.Local().Format("2006-01-02 15:04:05")
		} else {
			pending++
		}
		if s.Modified {
			modified++
		}

		reversible := "yes"
		if !s.Migration.Reversible() {
			reversible = "no"
		}

		fmt.Fprintf(out, "%d\t%s\t%s\t%s\t%s\n",
			s.Migration.Version, s.Migration.Name, s.State(), appliedAt, reversible)
	}
	out.Flush()

	fmt.Printf("\n%d migration(s), %d pending\n", len(report), pending)
	if modified > 0 {
		// A modified migration is why two databases can claim the same version
		// with different schemas, so it is called out rather than left in a
		// column somebody has to notice.
		fmt.Fprintf(os.Stderr,
			"\n%d migration(s) were changed after being applied; `migrate up` will refuse to run\n", modified)
		return 1
	}
	return 0
}

func migrateVersion(ctx context.Context, runner *db.Runner) int {
	report, err := runner.Status(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate version: %v\n", err)
		return 1
	}

	var latest string
	for _, s := range report {
		if s.Applied != nil {
			latest = s.Migration.String()
		}
	}
	if latest == "" {
		fmt.Println("no migrations have been applied")
		return 0
	}

	fmt.Println(latest)
	return 0
}

// optionalCount reads a trailing count argument, falling back to fallback.
func optionalCount(args []string, fallback int) (int, bool) {
	if len(args) == 0 {
		return fallback, true
	}

	count, err := strconv.Atoi(args[0])
	if err != nil || count < 1 {
		fmt.Fprintf(os.Stderr, "expected a positive number of migrations, got %q\n", args[0])
		return 0, false
	}
	return count, true
}
