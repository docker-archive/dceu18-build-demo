package util

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/containerd/console"
	"github.com/containerd/containerd/platforms"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	gateway "github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/util/progress/progressui"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

func WithProgressAndExport(ctx context.Context, buildkit, progress string, push bool, cb func(context.Context, *client.Client, io.WriteCloser, chan *client.SolveStatus) (*client.SolveResponse, error)) (*client.SolveResponse, error) {
	c, err := client.New(ctx, buildkit, client.WithFailFast())
	if err != nil {
		return nil, err
	}

	var (
		pipeR *io.PipeReader
		pipeW *io.PipeWriter
	)
	if !push {
		pipeR, pipeW = io.Pipe()
	}
	ch := make(chan *client.SolveStatus)

	eg, ctx := errgroup.WithContext(ctx)
	var res *client.SolveResponse
	eg.Go(func() error {
		var err error
		if res, err = cb(ctx, c, pipeW, ch); err != nil {
			if pipeW != nil {
				pipeW.CloseWithError(err)
			}
			return err
		}
		return nil
	})
	eg.Go(func() error {
		var c console.Console

		switch progress {
		case "auto", "tty":
			cf, err := console.ConsoleFromFile(os.Stderr)
			if err != nil && progress == "tty" {
				return err
			}
			c = cf
		case "plain":
		default:
			return errors.Errorf("invalid progress value : %s", progress)
		}

		// not using shared context to not disrupt display but let is finish reporting errors
		return progressui.DisplaySolveStatus(context.TODO(), "", c, os.Stdout, ch)
	})
	if !push {
		eg.Go(func() error {
			if err := loadDockerTar(pipeR); err != nil {
				return err
			}
			return pipeR.Close()
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return res, nil
}

func BuildWithProgressAndExport(ctx context.Context, buildkit, progress, tag string, push bool, cb gateway.BuildFunc, attrs map[string]string, localdirs map[string]string) (*client.SolveResponse, error) {
	return WithProgressAndExport(ctx, buildkit, progress, push, func(ctx context.Context, c *client.Client, pipeW io.WriteCloser, ch chan *client.SolveStatus) (*client.SolveResponse, error) {
		so := client.SolveOpt{
			FrontendAttrs: attrs,
			ExporterAttrs: map[string]string{
				"name": tag,
			},
			LocalDirs: localdirs,
			Session: []session.Attachable{
				authprovider.NewDockerAuthProvider(),
			},
		}
		if push {
			so.Exporter = "image"
			so.FrontendAttrs["push"] = "true"
			so.ExporterAttrs["push"] = "true"
		} else {
			so.Exporter = "docker"
			so.ExporterOutput = pipeW
		}
		return c.Build(ctx, so, "", cb, ch)
	})
}

func SolveWithProgressAndExport(ctx context.Context, buildkit, progress, tag, fe string, push bool, attrs map[string]string, localdirs map[string]string) (*client.SolveResponse, error) {
	return WithProgressAndExport(ctx, buildkit, progress, push, func(ctx context.Context, c *client.Client, pipeW io.WriteCloser, ch chan *client.SolveStatus) (*client.SolveResponse, error) {
		if attrs == nil {
			attrs = map[string]string{}
		}
		attrs["source"] = fe
		so := client.SolveOpt{
			Frontend:      "gateway.v0",
			FrontendAttrs: attrs,
			ExporterAttrs: map[string]string{
				"name": tag,
			},
			LocalDirs: localdirs,
			Session: []session.Attachable{
				authprovider.NewDockerAuthProvider(),
			},
		}
		if push {
			so.Exporter = "image"
			so.FrontendAttrs["push"] = "true"
			so.ExporterAttrs["push"] = "true"
		} else {
			so.Exporter = "docker"
			so.ExporterOutput = pipeW
		}
		return c.Solve(ctx, nil, so, ch)
	})
}

func loadDockerTar(r io.Reader) error {
	// no need to use moby/moby/client here
	cmd := exec.Command("docker", "load")
	cmd.Stdin = r
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Build(ctx context.Context, c gateway.Client, st llb.State, img ocispec.Image) (*gateway.Result, error) {
	def, err := st.Marshal()
	if err != nil {
		return nil, err
	}

	t := time.Now()
	img.Created = &t

	config, err := json.Marshal(img)
	if err != nil {
		return nil, err
	}

	res, err := c.Solve(ctx, gateway.SolveRequest{
		Definition: def.ToPB(),
	})
	if err != nil {
		return nil, err
	}

	res.AddMeta(exptypes.ExporterImageConfigKey, config)

	return res, err
}

func BuildForPlatforms(ctx context.Context, c gateway.Client, cb func(context.Context, gateway.Client, ocispec.Platform) (llb.State, ocispec.Image, error), ps ...ocispec.Platform) (*gateway.Result, error) {
	res := gateway.NewResult()
	eg, ctx := errgroup.WithContext(ctx)
	expPlatforms := &exptypes.Platforms{
		Platforms: make([]exptypes.Platform, len(ps)),
	}

	for i, p := range ps {
		func(i int, p ocispec.Platform) {
			eg.Go(func() error {
				st, img, err := cb(ctx, c, p)
				if err != nil {
					return err
				}

				def, err := st.Marshal()
				if err != nil {
					return err
				}

				t := time.Now()
				img.Created = &t

				config, err := json.Marshal(img)
				if err != nil {
					return err
				}

				r, err := c.Solve(ctx, gateway.SolveRequest{
					Definition: def.ToPB(),
				})
				if err != nil {
					return err
				}

				ref, err := r.SingleRef()
				if err != nil {
					return err
				}

				if len(ps) == 1 {
					res.AddMeta(exptypes.ExporterImageConfigKey, config)
					res.SetRef(ref)
				} else {
					k := platforms.Format(p)
					res.AddMeta(fmt.Sprintf("%s/%s", exptypes.ExporterImageConfigKey, k), config)
					res.AddRef(k, ref)
					expPlatforms.Platforms[i] = exptypes.Platform{
						ID:       k,
						Platform: p,
					}
				}
				return nil
			})
		}(i, p)
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	if len(ps) > 1 {
		dt, err := json.Marshal(expPlatforms)
		if err != nil {
			return nil, err
		}
		res.AddMeta(exptypes.ExporterPlatformsKey, dt)
	}
	return res, nil
}
