package streaming

import (
	"context"
	"time"

	"github.com/iancoleman/strcase"
	uuid "github.com/satori/go.uuid"
	"github.com/streadway/amqp"
	"go.uber.org/zap"
)

type Publisher struct { // TODO: add interface
	logger *zap.SugaredLogger
	cfg    *RabbitMQConfig
	conn   *amqp.Connection
	ctx    context.Context
}

func NewPublisher(logger *zap.SugaredLogger, cfg *RabbitMQConfig, conn *amqp.Connection, ctx context.Context) *Publisher {
	return &Publisher{
		logger: logger,
		cfg:    cfg,
		conn:   conn,
		ctx:    ctx,
	}
}

// TODO: add key
func (p *Publisher) Publish(ctx context.Context, destination string, data []byte) error {
	l := p.logger
	l.Infof("Publishing message to destination %s", destination)

	snakeTypeName := strcase.ToSnake(destination)
	channel, err := p.conn.Channel()
	if err != nil {
		l.Error("Error in opening channel to consume message")
		return err
	}
	err = channel.ExchangeDeclare(
		snakeTypeName, // name
		p.cfg.Kind,    // type
		true,          // durable
		false,         // auto-deleted
		false,         // internal
		false,         // no-wait
		nil,           // arguments
	)

	if err != nil {
		l.Error("Error in declaring exchange to publish message")
		return err
	}

	publishingMsg := amqp.Publishing{
		Body:          data,
		ContentType:   "application/json",
		DeliveryMode:  amqp.Persistent,
		MessageId:     uuid.NewV4().String(),
		Timestamp:     time.Now(),
		CorrelationId: "",
	}

	err = channel.Publish(snakeTypeName, snakeTypeName, false, false, publishingMsg)

	if err != nil {
		l.Error("Error in publishing message")
		return err
	}

	defer channel.Close()

	return nil
}
