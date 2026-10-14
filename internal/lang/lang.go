// Package lang defines the language adapter contract. Adapters turn one
// source file into model.FileFacts and resolve module specifiers.
package lang

import (
	"github.com/sadayamuthu/mergelock/internal/model"
	"github.com/sadayamuthu/mergelock/internal/verdict"
)

// FS is a read-only view of one revision's tree.
type FS interface {
	Exists(path string) bool
	Read(path string) ([]byte, bool)
}

type Adapter interface {
	Name() string
	Version() string
	Match(path string) bool
	Parse(path string, src []byte) (*model.FileFacts, error)
	ResolveModule(fromFile, spec string, fs FS) (string, bool)
	ConstructorName() string
	MaxConfidence() verdict.Confidence
	SubmoduleSpec(spec, name string) (string, bool)
}

type Registry struct{ adapters []Adapter }

func NewRegistry(adapters ...Adapter) *Registry {
	return &Registry{adapters: adapters}
}

// For returns the adapter for path, or nil when the language is unsupported.
func (r *Registry) For(path string) Adapter {
	for _, a := range r.adapters {
		if a.Match(path) {
			return a
		}
	}
	return nil
}

func (r *Registry) All() []Adapter { return r.adapters }
