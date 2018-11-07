package main

import (
	"flag"
	"fmt"
	"os"

	demo "dceu18-build-demo"
	"dceu18-build-demo/util"

	"github.com/moby/buildkit/util/appcontext"
	"github.com/moby/buildkit/util/appdefaults"
)

func main() {
	ctx := appcontext.Context()

	buildkit := flag.String("buildkit", appdefaults.Address, "address of buildkit server")
	progress := flag.String("progress", "auto", "style of progress meter")
	tag := flag.String("tag", "demo", "tag to use on image")
	localContext := flag.String("context", ".", "path to context")
	push := flag.Bool("push", false, "push the image")

	flag.Parse()

	localdirs := map[string]string{
		"context": *localContext,
	}
	_, err := util.BuildWithProgressAndExport(ctx, *buildkit, *progress, *tag, *push, demo.Build, nil, localdirs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}
