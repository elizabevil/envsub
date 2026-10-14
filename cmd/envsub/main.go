// Command envsub renders templates with the extended substitution syntax.
package main

import (
	"os"

	"github.com/elizabevil/envsub/internal/app"
)

func main() {
	os.Exit(run())
}

// run isolates the process exit code from main so it can be asserted by tests.
func run() int {
	return app.Run()
}
