package yaran

import (
	"fmt"
	"sort"
)

type ProgressFunc func(completed int, total *int)

type AddressInserter interface {
	Insert(address Address) (Address, error)
}

type FormatHandler interface {
	Format() string
	Import(path string, sink AddressInserter, progress ProgressFunc) error
	Export(path string, addresses []Address, progress ProgressFunc) error
}

type Registry struct {
	handlers map[string]FormatHandler
}

func NewRegistry() *Registry {
	registry := &Registry{
		handlers: map[string]FormatHandler{},
	}

	mustRegister := func(handler FormatHandler) {
		if err := registry.Register(handler); err != nil {
			panic(err)
		}
	}

	mustRegister(CSVPlugin{})
	mustRegister(JSONPlugin{})
	mustRegister(HTMLPlugin{})
	mustRegister(VCardPlugin{})

	return registry
}

func (r *Registry) Register(handler FormatHandler) error {
	format := handler.Format()
	if _, exists := r.handlers[format]; exists {
		return fmt.Errorf("plugin already registered: %s", format)
	}
	r.handlers[format] = handler
	return nil
}

func (r *Registry) Get(format string) (FormatHandler, error) {
	handler, ok := r.handlers[format]
	if !ok {
		return nil, fmt.Errorf("unknown file format: %s", format)
	}
	return handler, nil
}

func (r *Registry) AvailableFormats() []string {
	formats := make([]string, 0, len(r.handlers))
	for format := range r.handlers {
		formats = append(formats, format)
	}
	sort.Strings(formats)
	return formats
}

func reportProgress(progress ProgressFunc, completed int, total *int) {
	if progress != nil {
		progress(completed, total)
	}
}

func intRef(value int) *int {
	return &value
}
