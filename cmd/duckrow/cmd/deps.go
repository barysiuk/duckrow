package cmd

import (
	"fmt"

	"github.com/barysiuk/duckrow/internal/core"
)

// deps holds shared dependencies for CLI commands.
type deps struct {
	config *core.ConfigManager
}

// newDeps creates shared dependencies. Called lazily by commands that need them.
func newDeps() (*deps, error) {
	config, err := core.NewConfigManager()
	if err != nil {
		return nil, fmt.Errorf("initializing config: %w", err)
	}

	return &deps{
		config: config,
	}, nil
}
