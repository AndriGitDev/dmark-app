package config

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	HTTPAddr              string   `env:"HTTP_ADDR" envDefault:":8080"`
	PublicBaseURL         string   `env:"PUBLIC_BASE_URL" envDefault:"http://localhost:8080"`
	DatabaseURL           string   `env:"DATABASE_URL" envDefault:"postgres://meil:meil@localhost:5432/meil?sslmode=disable"`
	ClickHouseURL         string   `env:"CLICKHOUSE_URL" envDefault:"http://localhost:8123"`
	ClickHouseDatabase    string   `env:"CLICKHOUSE_DATABASE" envDefault:"meil"`
	ClickHouseUser        string   `env:"CLICKHOUSE_USER" envDefault:"meil"`
	ClickHousePassword    string   `env:"CLICKHOUSE_PASSWORD" envDefault:"meil"`
	LettermintBaseURL     string   `env:"LETTERMINT_BASE_URL" envDefault:"https://api.lettermint.co/v1"`
	LettermintToken       string   `env:"LETTERMINT_TOKEN"`
	LettermintFrom        string   `env:"LETTERMINT_FROM"`
	AuthTokenTTL          int      `env:"AUTH_TOKEN_TTL_MINUTES" envDefault:"15"`
	BillingWebhookSecret  string   `env:"BILLING_WEBHOOK_SECRET"`
	FrontendOrigins       []string `env:"FRONTEND_ORIGINS" envSeparator:","`
	PublicChecksPerMinute int      `env:"PUBLIC_CHECKS_PER_MINUTE" envDefault:"12"`
	DMARCMaxReportBytes   int64    `env:"DMARC_MAX_REPORT_BYTES" envDefault:"10485760"`
}

func Load() (Config, error) {
	return env.ParseAs[Config]()
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.HTTPAddr) == "" {
		return fmt.Errorf("HTTP_ADDR is required")
	}
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	databaseURL, err := url.Parse(c.DatabaseURL)
	if err != nil || databaseURL.Scheme == "" {
		return fmt.Errorf("DATABASE_URL is invalid")
	}
	clickhouseURL, err := url.Parse(c.ClickHouseURL)
	if err != nil || clickhouseURL.Scheme == "" || clickhouseURL.Host == "" {
		return fmt.Errorf("CLICKHOUSE_URL is invalid")
	}
	if strings.TrimSpace(c.ClickHouseDatabase) == "" {
		return fmt.Errorf("CLICKHOUSE_DATABASE is required")
	}
	publicBaseURL, err := url.Parse(c.PublicBaseURL)
	if err != nil || publicBaseURL.Scheme == "" || publicBaseURL.Host == "" {
		return fmt.Errorf("PUBLIC_BASE_URL must be an absolute URL")
	}
	if c.AuthTokenTTL <= 0 {
		return fmt.Errorf("AUTH_TOKEN_TTL_MINUTES must be greater than zero")
	}
	for _, origin := range c.FrontendOrigins {
		parsed, err := url.Parse(strings.TrimSpace(origin))
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return fmt.Errorf("FRONTEND_ORIGINS must contain absolute origins only")
		}
	}
	if c.PublicChecksPerMinute <= 0 {
		return fmt.Errorf("PUBLIC_CHECKS_PER_MINUTE must be greater than zero")
	}
	if c.DMARCMaxReportBytes <= 0 {
		return fmt.Errorf("DMARC_MAX_REPORT_BYTES must be greater than zero")
	}
	return nil
}
