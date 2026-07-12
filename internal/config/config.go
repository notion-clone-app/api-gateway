package config

import (
	"flag"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Auth      AuthConfig                `yaml:"auth"`
	Env       string                    `yaml:"env" env-required:"true"`
	Upstreams map[string]UpstreamConfig `yaml:"upstreams" env-required:"true"`
	HTTP      HttpConfig                `yaml:"http" env-required:"true"`
}

type AuthConfig struct {
	HMACSecret string `yaml:"hmac_secret" env:"JWT_SECRET"`
	Issuer     string `yaml:"issuer" env:"JWT_ISSUER"`
	Audience   string `yaml:"audience" env:"JWT_AUDIENCE"`
}

type HttpConfig struct {
	Port    string        `yaml:"port"`
	Timeout time.Duration `yaml:"timeout"`
}

type UpstreamConfig struct {
	GRPCAddress string        `yaml:"grpc_address"`
	Timeout     time.Duration `yaml:"timeout"`
}

func MustLoad() *Config {
	path := fetchConfigPath()
	if path == "" {
		panic("config path is empty")
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		panic("config gile does not exist" + path)
	}

	var cfg Config

	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		panic("failed to load config" + err.Error())
	}

	return &cfg
}

func fetchConfigPath() string {
	var res string

	// --config="path/to/config.yaml"
	flag.StringVar(&res, "config", "", "path to config file")
	flag.Parse()

	if res == "" {
		res = os.Getenv("CONFIG_PATH")
	}

	return res
}
