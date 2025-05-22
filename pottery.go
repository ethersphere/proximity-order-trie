package proximityordertrie

import (
	"context"
	"errors"

	"github.com/nugaon/proximity-order-trie/pkg/persister"
	"github.com/nugaon/proximity-order-trie/pkg/pot"
)

var (
	// ErrFeatureNotFound returned when a feature (by name) is not found
	ErrFeatureNotFound = errors.New("feature not found")
	// ErrFacetNotFound returned when a facet (by name) is not found
	ErrFaceNotFound = errors.New("face not found")
)

// Pottery is a set of related faceted object relational mappings
type Pottery struct {
	forms map[string]*FORM
}

// NewPottery is the constructor of a pottery which are based on a schema and has facets
func NewPottery(dir string, schema *Schema, facets []Facet, ls persister.LoadSaver) (*Pottery, error) {
	forms := make(map[string]*FORM)
	for _, f := range facets {
		key, err := schema.Slice(f.Key)
		if err != nil {
			return nil, err
		}
		val, err := schema.Slice(f.Val)
		if err != nil {
			return nil, err
		}
		mode := pot.NewPersistedPot(pot.NewSingleOrder(8*key.Size()), ls, func() pot.Entry { return &Entry{} })
		idx, err := New(mode)
		if err != nil {
			return nil, err
		}
		forms[f.Name] = &FORM{
			Facet: f,
			key:   key,
			val:   val,
			mode:  mode,
			pot:   idx,
		}
	}
	return &Pottery{forms}, nil
}

// Find in a particular facet
func (p *Pottery) Find(ctx context.Context, name string, r *Record) error {
	f, ok := p.forms[name]
	if !ok {
		return ErrFaceNotFound
	}
	return f.Find(ctx, r)
}

// Find in a particular facet
func (p *Pottery) Iterate(ctx context.Context, name string, filterBy, startFrom *Record, create func() *Record, g func(*Record) (stop bool, err error)) error {
	f, ok := p.forms[name]
	if !ok {
		return ErrFaceNotFound
	}
	return f.Iterate(ctx, filterBy, startFrom, create, g)
}

// Add inserts an entry to the mutable pot
func (p Pottery) Add(ctx context.Context, r *Record) error {
	for _, f := range p.forms {
		if err := f.Add(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

// Delete removes an entry from the mutable pot
func (p Pottery) Delete(ctx context.Context, r *Record) error {
	for _, f := range p.forms {
		if err := f.Delete(ctx, r); err != nil {
			return err
		}
	}
	return nil
}
