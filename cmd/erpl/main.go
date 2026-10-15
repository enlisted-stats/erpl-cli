package main

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"

	"github.com/enlisted-stats/erpl-cli/internal/cli"
)

// version is set by the release build, e.g. to "v1.0.0"; a build from source reports none.
var version string

func main() {
	if err := fang.Execute(context.Background(), cli.New(), fang.WithVersion(version), fang.WithoutCompletions(), fang.WithoutManpage()); err != nil {
		os.Exit(1)
	}
}
