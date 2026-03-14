package backend

import "context"

// Lookup exposes the product-level lookup operation: does stem S exist in zone Z?
type Lookup interface {
	ZoneNames() []string
	Contains(ctx context.Context, zone, stem string) (bool, error)
}
