package backend

import (
	"context"
	"fmt"

	"github.com/genestevens/domain-finder/internal/index"
)

// File uses loaded zone files for exact-match lookups.
type File struct {
	multi *index.Multi
}

// LoadFile constructs a file-backed lookup from explicit zone=path inputs.
func LoadFile(zones map[string]string) (*File, error) {
	multi, err := index.LoadMulti(zones)
	if err != nil {
		return nil, err
	}
	return &File{multi: multi}, nil
}

func (f *File) ZoneNames() []string {
	if f == nil || f.multi == nil {
		return nil
	}
	return f.multi.ZoneNames()
}

func (f *File) Contains(_ context.Context, zone, stem string) (bool, error) {
	if f == nil || f.multi == nil {
		return false, fmt.Errorf("file backend is not initialized")
	}
	return f.multi.Contains(zone, composeLookupName(stem, zone)), nil
}

func composeLookupName(stem, zone string) string {
	return fmt.Sprintf("%s.%s", stem, zone)
}
