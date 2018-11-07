package demo

import (
	"context"

	gateway "github.com/moby/buildkit/frontend/gateway/client"
)

func Build(ctx context.Context, c gateway.Client) (*gateway.Result, error) {
	return gateway.NewResult(), nil
}
