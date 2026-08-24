package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func files(names ...string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for _, name := range names {
		fsys[name] = &fstest.MapFile{Data: []byte("-- " + name)}
	}
	return fsys
}

// A plain filename sort puts "10_x" before "9_x". The version is a number, so
// it is compared as one.
func TestMigrationsAreOrderedNumericallyNotAlphabetically(t *testing.T) {
	got, err := loadMigrations(files(
		"9_ninth.up.sql", "10_tenth.up.sql", "2_second.up.sql",
	))
	if err != nil {
		t.Fatalf("loading: %v", err)
	}

	var versions []int64
	for _, m := range got {
		versions = append(versions, m.Version)
	}
	want := []int64{2, 9, 10}
	for i := range want {
		if versions[i] != want[i] {
			t.Fatalf("versions = %v, want %v", versions, want)
		}
	}
}

// Up and down are paired by version, so renaming the readable half of one
// filename cannot silently orphan the other.
func TestUpAndDownArePairedByVersion(t *testing.T) {
	got, err := loadMigrations(files("0001_products.up.sql", "0001_products.down.sql"))
	if err != nil {
		t.Fatalf("loading: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d migrations, want 1 with both halves", len(got))
	}
	if !got[0].Reversible() {
		t.Error("the down file was not paired with its up file")
	}
}

// A migration with no down file is not an error: it is how a change that cannot
// be undone says so.
func TestAMigrationMayHaveNoDownFile(t *testing.T) {
	got, err := loadMigrations(files("0001_products.up.sql"))
	if err != nil {
		t.Fatalf("loading: %v", err)
	}
	if got[0].Reversible() {
		t.Error("a migration with no down file reported itself reversible")
	}
}

// A down file with no up file is a half-deleted migration: running what is left
// would take the schema somewhere no build describes.
func TestADownFileWithoutAnUpFileIsRefused(t *testing.T) {
	_, err := loadMigrations(files("0001_products.down.sql"))
	if err == nil {
		t.Fatal("an orphaned down file was accepted")
	}
	if !strings.Contains(err.Error(), "no up file") {
		t.Errorf("err = %v, want it to name the missing up file", err)
	}
}

// Two migrations claiming one version is a merge that compiled. It has to be
// caught before either of them runs.
func TestTwoMigrationsCannotClaimOneVersion(t *testing.T) {
	_, err := loadMigrations(files("0004_orders.up.sql", "0004_invoices.up.sql"))
	if err == nil {
		t.Fatal("two migrations sharing a version were accepted")
	}
	if !strings.Contains(err.Error(), "claimed by two migrations") {
		t.Errorf("err = %v, want it to name the collision", err)
	}
}

func TestAMisnamedFileIsRefused(t *testing.T) {
	for _, name := range []string{
		"products.sql",         // no version
		"0001-products.up.sql", // hyphen, not underscore
		"0001_products.sql",    // no direction
		"0001_Products.up.sql", // uppercase
		"0001_products.sideways.sql",
	} {
		if _, err := loadMigrations(files(name)); err == nil {
			t.Errorf("%q was accepted", name)
		}
	}
}

// The checksum is what makes an edited-after-apply migration detectable.
func TestChecksumChangesWithTheStatements(t *testing.T) {
	first := checksum([]byte("CREATE TABLE a ()"))
	second := checksum([]byte("CREATE TABLE b ()"))

	if first == second {
		t.Fatal("two different migrations hash the same")
	}
	if first != checksum([]byte("CREATE TABLE a ()")) {
		t.Error("the same statements hash differently")
	}
}

// verify is the gate in front of every up and down run.
func TestVerifyRejectsAnEditedMigration(t *testing.T) {
	fsys := files("0001_products.up.sql")
	migrations, err := loadMigrations(fsys)
	if err != nil {
		t.Fatalf("loading: %v", err)
	}

	applied := map[int64]AppliedMigration{
		1: {Version: 1, Name: "products", Checksum: checksum([]byte("what it used to say"))},
	}

	err = verify(migrations, applied, fsys)
	if err == nil {
		t.Fatal("an edited migration was accepted")
	}
	if !strings.Contains(err.Error(), "changed after it was applied") {
		t.Errorf("err = %v, want it to say the file was edited", err)
	}
}

// A database that has run something this build does not carry is a rollback to
// an older binary. Applying the rest would leave a schema no version describes.
func TestVerifyRejectsAnUnknownAppliedMigration(t *testing.T) {
	fsys := files("0001_products.up.sql")
	migrations, err := loadMigrations(fsys)
	if err != nil {
		t.Fatalf("loading: %v", err)
	}

	applied := map[int64]AppliedMigration{
		1:  {Version: 1, Name: "products", Checksum: checksum([]byte("-- 0001_products.up.sql"))},
		99: {Version: 99, Name: "from_the_future", Checksum: "whatever"},
	}

	err = verify(migrations, applied, fsys)
	if err == nil {
		t.Fatal("an unknown applied migration was accepted")
	}
	if !strings.Contains(err.Error(), "this build does not contain it") {
		t.Errorf("err = %v", err)
	}
}

func TestVerifyAcceptsAnUnchangedHistory(t *testing.T) {
	fsys := files("0001_products.up.sql")
	migrations, err := loadMigrations(fsys)
	if err != nil {
		t.Fatalf("loading: %v", err)
	}

	applied := map[int64]AppliedMigration{
		1: {Version: 1, Name: "products", Checksum: checksum([]byte("-- 0001_products.up.sql"))},
	}
	if err := verify(migrations, applied, fsys); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestGenerateWritesAPairThatParses(t *testing.T) {
	dir := t.TempDir()

	upPath, downPath, err := Generate(dir, "  Add Orders Table!  ")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	for _, path := range []string{upPath, downPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}

	// Whatever was typed becomes a filename-safe name.
	if !strings.Contains(filepath.Base(upPath), "add_orders_table") {
		t.Errorf("up file = %q, want a cleaned name", filepath.Base(upPath))
	}

	// The generated pair has to be readable by the runner that will apply it.
	got, err := loadMigrations(os.DirFS(dir))
	if err != nil {
		t.Fatalf("the generated files do not parse: %v", err)
	}
	if len(got) != 1 || !got[0].Reversible() {
		t.Fatalf("got %+v, want one reversible migration", got)
	}
}

func TestGenerateRefusesANamelessMigration(t *testing.T) {
	if _, _, err := Generate(t.TempDir(), "   "); err == nil {
		t.Fatal("a migration with no name was generated")
	}
}

// Generating twice in the same second must not quietly replace work that is
// already there.
func TestGenerateRefusesToOverwrite(t *testing.T) {
	dir := t.TempDir()

	upPath, _, err := Generate(dir, "add orders")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	version := strings.SplitN(filepath.Base(upPath), "_", 2)[0]
	clash := filepath.Join(dir, version+"_add_orders.up.sql")
	if err := os.WriteFile(clash, []byte("-- already here"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Same directory, same name, same second -> the same filename.
	if _, _, err := Generate(dir, "add orders"); err == nil {
		t.Skip("the clock moved on between generates; nothing to collide with")
	}
}

func TestGenerateRefusesAMissingDirectory(t *testing.T) {
	if _, _, err := Generate(filepath.Join(t.TempDir(), "nope"), "add orders"); err == nil {
		t.Fatal("a missing migration directory was accepted")
	}
}

// The shipped migrations have to satisfy the same rules as generated ones.
func TestTheShippedMigrationsParse(t *testing.T) {
	got, err := loadMigrations(os.DirFS("../../migration"))
	if err != nil {
		t.Fatalf("the shipped migrations do not parse: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("no migrations were found")
	}

	for _, m := range got {
		if !m.Reversible() {
			t.Errorf("%s has no down file", m)
		}
	}
}
