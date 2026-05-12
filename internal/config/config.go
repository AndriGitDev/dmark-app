// Package config loads process configuration from the environment.
package config

import "github.com/caarlos0/env/v11"

type Config struct {
	HTTPAddr      string `env:"HTTP_ADDR" envDefault:":8080"`
	PublicBaseURL string `env:"PUBLIC_BASE_URL" envDefault:"http://localhost:8080"`
	AssetsDir     string `env:"ASSETS_DIR" envDefault:"assets"`
}

func Load() (Config, error) {
	return env.ParseAs[Config]()
}
