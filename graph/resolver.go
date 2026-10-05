package graph

import "github.com/RDX-Rajat-Savdekar/pulse/internal/querypb"

// Resolver is the gqlgen dependency root. QueryClient is the internal gRPC read API.
type Resolver struct {
	QueryClient querypb.QueryServiceClient
}
