package builtin

import (
	"errors"

	"neo-code/internal/provider"
	"neo-code/internal/provider/anthropic"
	"neo-code/internal/provider/gemini"
	"neo-code/internal/provider/openaicompat"
)

func NewRegistry() (*provider.Registry, error) {
	registry := provider.NewRegistry()
	if err := register(registry); err != nil {
		return nil, err
	}
	return registry, nil
}

func register(registry *provider.Registry) error {
	if registry == nil {
		return errors.New("builtin provider registry is nil")
	}
	if err := registry.Register(openaicompat.Driver()); err != nil {
		return err
	}
	if err := registry.Register(gemini.Driver()); err != nil {
		return err
	}
	return registry.Register(anthropic.Driver())
}
