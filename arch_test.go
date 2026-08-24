package main

import (
	"os/exec"
	"sort"
	"strings"
	"testing"
)

// The layering rule, enforced rather than documented.
//
// A package may import only packages strictly below it. Nothing may import
// sideways or downward: a parent never reaches into a child, and a child reaches
// only up towards domain. Because every arrow points the same way, the graph
// cannot contain a cycle, and no layer can be replaced from underneath by the
// one it serves.
//
// The numbers are gaps-of-ten so a package can be slotted between two layers
// without renumbering the rest.
var layers = map[string]int{
	// The centre: entities and the rules over them, plus the process's own
	// settings. Nothing in here knows this program has an HTTP interface or a
	// database.
	"domain": 0,
	"config": 0,
	// The schema. It is data, not code: an embed.FS and a directory name, so it
	// can sit at the centre without dragging anything in.
	"migration": 0,

	// Mechanism with no knowledge of any aggregate: the reply envelope and the
	// SQL plumbing. Shared this widely, either one would hand its dependencies
	// to every package in the tree.
	"rest/response":    10,
	"infra/db/dbquery": 10,

	// The aggregates. Each owns its rules and declares the ports it needs in
	// port.go; none may see another.
	"auth":    20,
	"product": 20,
	"user":    20,

	// Adapters for the ports that are not storage, and the shared transport
	// pieces.
	"infra/db":         30,
	"infra/security":   30,
	"rest/helpers":     30,
	"rest/middlewares": 30,

	// Storage adapters, one package per aggregate.
	"repo/productrepo": 40,
	"repo/userrepo":    40,
	"repo/authrepo":    40,

	// Transport for each resource.
	"rest/handlers/root":    50,
	"rest/handlers/product": 50,
	"rest/handlers/user":    50,
	"rest/handlers/auth":    50,

	// The server, which mounts registrars it knows nothing about.
	"rest": 60,

	// The composition root, and main.
	"cmd": 70,
	"":    80,
}

// aggregates are the bounded contexts.
var aggregates = []string{"auth", "product", "user"}

// mayImport says which aggregates each package is entitled to see. A package
// absent from this table may see none.
//
// The zero-aggregate default is the load-bearing part. A package shared by
// several aggregates' code -- the reply envelope, the request helpers, the SQL
// plumbing -- hands its own dependencies to everything that uses it, so one
// aggregate import there quietly makes every other handler depend on that
// aggregate. Layer numbers do not catch it, because the aggregate genuinely does
// sit below the shared package. Only saying "this one is allowed nothing" does.
var mayImport = map[string][]string{
	// Storage and transport for one aggregate each.
	"repo/productrepo":      {"product"},
	"repo/userrepo":         {"user"},
	"repo/authrepo":         {"auth"},
	"rest/handlers/product": {"product"},
	"rest/handlers/user":    {"user"},
	"rest/handlers/auth":    {"auth"},

	// The composition root exists to join them, so it sees all three.
	composition: aggregates,
}

// composition is the one package allowed to see more than one aggregate.
const composition = "cmd"

const module = "go-commerce"

func TestLayeringIsRespected(t *testing.T) {
	for pkg, imports := range internalImports(t) {
		from, known := layers[pkg]
		if !known {
			t.Errorf("package %q is not placed in a layer; add it to arch_test.go so "+
				"its dependencies are checked", pkg)
			continue
		}

		for _, imported := range imports {
			to, known := layers[imported]
			if !known {
				continue // reported above, on its own iteration
			}

			if to >= from {
				t.Errorf("%s (layer %d) imports %s (layer %d): a package may only "+
					"import a layer strictly below it", label(pkg), from, label(imported), to)
			}
		}
	}
}

func TestDomainDependsOnNothing(t *testing.T) {
	// The centre has to be reachable from everywhere and reach nothing, or the
	// rules in it end up shaped by whatever it was allowed to import.
	if imports := internalImports(t)["domain"]; len(imports) > 0 {
		t.Errorf("domain imports %v; it must depend on nothing in this module", imports)
	}
}

func TestEachPackageSeesOnlyItsOwnAggregate(t *testing.T) {
	for pkg, imports := range internalImports(t) {
		allowed := make(map[string]bool, len(aggregates))
		for _, aggregate := range mayImport[pkg] {
			allowed[aggregate] = true
		}

		var reached []string
		for _, imported := range imports {
			// Match an aggregate itself, not a package that merely lives under a
			// similarly named path.
			if isAggregate(imported) && !allowed[imported] {
				reached = append(reached, imported)
			}
		}
		sort.Strings(reached)

		if len(reached) == 0 {
			continue
		}

		switch {
		case isAggregate(pkg):
			t.Errorf("aggregate %q imports aggregate(s) %v: aggregates must not see "+
				"each other, so what one needs from another belongs in its port.go",
				pkg, reached)

		case len(mayImport[pkg]) > 0:
			t.Errorf("%s owns %v but also imports %v: an adapter or handler serves "+
				"one aggregate", label(pkg), mayImport[pkg], reached)

		default:
			t.Errorf("%s imports %v but is shared: whatever it hands back must be "+
				"aggregate-agnostic, or every other package using it inherits that "+
				"dependency", label(pkg), reached)
		}
	}
}

func isAggregate(pkg string) bool {
	for _, aggregate := range aggregates {
		if pkg == aggregate {
			return true
		}
	}
	return false
}

func TestServerKnowsNoHandlers(t *testing.T) {
	// The server mounts anything satisfying RouteRegistrar. If it imported a
	// handler package, adding a resource would mean editing the server.
	for _, imported := range internalImports(t)["rest"] {
		if strings.HasPrefix(imported, "rest/handlers/") {
			t.Errorf("rest imports %s; resources must arrive through NewServer as "+
				"registrars, not be named by the server", label(imported))
		}
	}
}

// internalImports maps every package in the module to the packages it imports
// from this module, with the module prefix stripped. Reading it from `go list`
// means the test checks what the compiler actually sees, not a hand-kept list.
func internalImports(t *testing.T) map[string][]string {
	t.Helper()

	out, err := exec.Command("go", "list", "-f",
		`{{.ImportPath}}|{{join .Imports " "}}`, "./...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}

	graph := make(map[string][]string)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		path, rawImports, found := strings.Cut(line, "|")
		if !found {
			continue
		}

		var internal []string
		for _, imported := range strings.Fields(rawImports) {
			if trimmed, ok := trimModule(imported); ok {
				internal = append(internal, trimmed)
			}
		}
		graph[mustTrim(path)] = internal
	}

	if len(graph) == 0 {
		t.Fatal("go list returned no packages")
	}
	return graph
}

func trimModule(path string) (string, bool) {
	if path == module {
		return "", true
	}
	if trimmed, ok := strings.CutPrefix(path, module+"/"); ok {
		return trimmed, true
	}
	return "", false
}

func mustTrim(path string) string {
	trimmed, _ := trimModule(path)
	return trimmed
}

// label names the root package readably, since its trimmed path is empty.
func label(pkg string) string {
	if pkg == "" {
		return "main"
	}
	return pkg
}
