package env

import "github.com/caarlos0/env"

type config struct {
	ServiceName        string `env:"SERVICE_NAME" envDefault:"orchestrator"`
	DBConnectionString string `env:"DB_CONNECTION_STRING" envDefault:"postgres://sagas:sagas@localhost:5432/sagas"`
	RedisAddr          string `env:"REDIS_ADDR" envDefault:"localhost:6379"`
	Topics             string `env:"TOPICS" envDefault:"service.orders.events,service.customers.events,service.accounting.events"`
}

func Load() (*config, error) {
	cfg := &config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
