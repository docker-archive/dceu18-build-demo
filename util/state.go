package util

import (
	"context"

	"github.com/moby/buildkit/client/llb"
	gateway "github.com/moby/buildkit/frontend/gateway/client"
)

func ReadFromState(ctx context.Context, c gateway.Client, st llb.State, filename string) ([]byte, error) {
	def, err := st.Marshal()
	if err != nil {
		return nil, err
	}

	res, err := c.Solve(ctx, gateway.SolveRequest{
		Definition: def.ToPB(),
	})
	if err != nil {
		return nil, err
	}

	ref, err := res.SingleRef()
	if err != nil {
		return nil, err
	}
	return ref.ReadFile(ctx, gateway.ReadRequest{Filename: filename})
}
