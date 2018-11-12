package demo

import (
	"context"

	"dceu18-build-demo/util"

	"github.com/moby/buildkit/client/llb"
	gateway "github.com/moby/buildkit/frontend/gateway/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func whalesay(p ocispec.Platform) llb.State {
	// Starting from an Alpine base image for the requested platform
	st := llb.Image("alpine:latest", llb.Platform(p))
	// Install the tools we need to run cowsay
	st = st.Run(llb.Args([]string{"apk", "add", "-U", "perl"})).Root()

	// Run the cowsay `install.sh` script, by mounting the cowsay
	// Git repo source repository directly onto /cowsay.
	cowsay := llb.Git("https://github.com/schacon/cowsay.git", "master")
	st = st.Run(
		llb.AddMount("/cowsay", cowsay),
		llb.Dir("/cowsay"),
		llb.Args([]string{"./install.sh", "/usr/local"}),
	).Root()

	// Now copy our custom artwork from the original
	// (linux/amd64-only) image using a helper image. We mount the
	// source image on /src and the destination image (which is
	// the cosway image we created above) onto /dst.  We also use
	// `llb.SourcePath` to mount the desired subpath of both
	// source and dest.
	busybox := "busybox:latest"
	whalesay := llb.Image("docker/whalesay:latest", llb.LinuxAmd64)
	cp := llb.Image(busybox).Run(
		llb.ReadonlyRootFS(),
		llb.Args([]string{"cp", "/src/usr/local/share/cows/docker.cow", "/dst/usr/local/share/cows/default.cow"}),
		llb.AddMount("/src", whalesay, llb.Readonly),
	)
	st = cp.AddMount("/dst", st)

	return st
}

func build(ctx context.Context, c gateway.Client, p ocispec.Platform) (llb.State, ocispec.Image, error) {
	txt := llb.Local("context", llb.IncludePatterns([]string{"hi.txt"}))
	bytes, err := util.ReadFromState(ctx, c, txt, "hi.txt")
	if err != nil {
		return llb.State{}, ocispec.Image{}, err
	}
	hi := string(bytes)

	st := whalesay(p)

	img := ocispec.Image{
		Architecture: p.Architecture,
		OS:           p.OS,
	}
	img.Config.Cmd = []string{"/usr/local/bin/cowsay", hi}

	return st, img, nil
}

func Build(ctx context.Context, c gateway.Client) (*gateway.Result, error) {
	p := ocispec.Platform{OS: "linux", Architecture: "amd64"}
	st, img, err := build(ctx, c, p)
	if err != nil {
		return nil, err
	}

	res, err := util.Build(ctx, c, st, img)
	if err != nil {
		return nil, err
	}

	return res, nil
}
