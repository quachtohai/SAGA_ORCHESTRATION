package streaming

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/cenkalti/backoff/v4"

	"github.com/streadway/amqp"
)

type RabbitMQConfig struct {
	Host         string
	Port         int
	User         string
	Password     string
	ExchangeName string
	Kind         string
}

func NewRabbitMQConfig(host string, port int, user string, password string, exchangename string, kind string) *RabbitMQConfig {
	return &RabbitMQConfig{Host: host, Port: port, User: user, Password: password, ExchangeName: exchangename, Kind: kind}
}

func NewRabbitMQConn(cfg *RabbitMQConfig, ctx context.Context) (*amqp.Connection, error) {
	connAddr := fmt.Sprintf(
		"amqp://%s:%s@%s:%d/", cfg.User, cfg.Password, cfg.Host, cfg.Port,
	)
	bo := backoff.NewExponentialBackOff()
	bo.MaxElapsedTime = 10 * time.Second
	maxRetries := 5

	var conn *amqp.Connection
	var err error

	err = backoff.Retry(func() error {
		conn, err = amqp.Dial(connAddr)
		if err != nil {
			log.Errorf("Failed to connect to RabbitMQ: %v. Connection information: %s", err, connAddr)
			return err
		}
		return nil

	}, backoff.WithMaxRetries(bo, uint64(maxRetries-1)))
	log.Info("Connected to RabbitMQ")

	go func() {
		select {
		case <-ctx.Done():
			err := conn.Close()
			if err != nil {
				log.Error("Failed to close RabbitMQ connection")
			}
			log.Info("RabbitMQ connection is closed")
		}
	}()
	return conn, err
}
