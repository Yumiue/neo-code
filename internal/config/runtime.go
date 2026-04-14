package config

import (
	"errors"
)

const (
	DefaultMaxNoProgressStreak = 3
)

type RuntimeConfig struct {
	MaxNoProgressStreak int `yaml:"max_no_progress_streak,omitempty"`
}

func defaultRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		MaxNoProgressStreak: DefaultMaxNoProgressStreak,
	}
}

func (c RuntimeConfig) Clone() RuntimeConfig {
	return c
}

func (c *RuntimeConfig) ApplyDefaults(defaults RuntimeConfig) {
	if c == nil {
		return
	}
	if c.MaxNoProgressStreak <= 0 {
		c.MaxNoProgressStreak = defaults.MaxNoProgressStreak
	}
}

func (c RuntimeConfig) Validate() error {
	if c.MaxNoProgressStreak <= 0 {
		return errors.New("max_no_progress_streak must be greater than 0")
	}
	return nil
}
