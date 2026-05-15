// Package config loads process configuration from the environment.
package config

import "github.com/caarlos0/env/v11"

type Config struct {
	HTTPAddr          string `env:"HTTP_ADDR" envDefault:":8080"`
	PublicBaseURL     string `env:"PUBLIC_BASE_URL" envDefault:"http://localhost:8080"`
	AssetsDir         string `env:"ASSETS_DIR" envDefault:"assets"`
	DatabaseURL       string `env:"DATABASE_URL" envDefault:"postgres://meil:meil@localhost:5432/meil?sslmode=disable"`
	LettermintBaseURL string `env:"LETTERMINT_BASE_URL" envDefault:"https://api.lettermint.co/v1"`
	LettermintToken   string `env:"LETTERMINT_TOKEN"`
	LettermintFrom    string `env:"LETTERMINT_FROM"`
	AuthTokenTTL      int    `env:"AUTH_TOKEN_TTL_MINUTES" envDefault:"15"`
}

func Load() (Config, error) {
	return env.ParseAs[Config]()
}
