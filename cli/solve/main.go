package main

import (
	"flag"
	"fmt"
	"os"

	"dceu18-build-demo/util"

	"github.com/moby/buildkit/util/appcontext"
	"github.com/moby/buildkit/util/appdefaults"
)

func main() {
	ctx := appcontext.Context()

	buildkit := flag.String("buildkit", appdefaults.Address, "address of buildkit server")
	progress := flag.String("progress", "auto", "style of progress meter")
	fe := flag.String("frontend", "localhost:5000/demo-frontend:latest", "frontend to call")
	tag := flag.String("tag", "demo", "tag to use on image")
	localContext := flag.String("context", ".", "path to context")
	push := flag.Bool("push", false, "push the image")

	flag.Parse()

	localdirs := map[string]string{
		"context": *localContext,
	}
	_, err := util.SolveWithProgressAndExport(ctx, *buildkit, *progress, *tag, *fe, *push, nil, localdirs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}
