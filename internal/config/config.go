// Package config loads process configuration from the environment.
package config

import "github.com/caarlos0/env/v11"

type Config struct {
	HTTPAddr string `env:"HTTP_ADDR" envDefault:":8080"`
}

func Load() (Config, error) {
	return env.ParseAs[Config]()
}
