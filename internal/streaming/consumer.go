package streaming

import (
	"context"
	"fmt"

	"github.com/iancoleman/strcase"
	"github.com/streadway/amqp"
	"go.uber.org/zap"
)

type Handler interface {
	Handle(ctx context.Context, msg []byte, commitFn func() error) error
}

type Consumer struct {
	logger  *zap.SugaredLogger
	cfg     *RabbitMQConfig
	conn    *amqp.Connection
	running bool
	handler Handler
	ctx     context.Context
	topic   string
}

func NewConsumer(logger *zap.SugaredLogger, cfg *RabbitMQConfig, conn *amqp.Connection, handler Handler, ctx context.Context, topic string) (*Consumer, error) {

	return &Consumer{
		logger:  logger,
		cfg:     cfg,
		conn:    conn,
		handler: handler,
		running: false,
		ctx:     ctx,
		topic:   topic,
	}, nil
}

func (c *Consumer) Start(ctx context.Context) (err error) {
	l := c.logger
	l.Info("Starting consumer")
	ch, err := c.conn.Channel()
	if err != nil {
		l.Error("Error in opening channel to consume message")
		return err
	}

	snakeTypeName := strcase.ToSnake(c.topic)

	err = ch.ExchangeDeclare(
		snakeTypeName, // name
		c.cfg.Kind,    // type
		true,          // durable
		false,         // auto-deleted
		false,         // internal
		false,         // no-wait
		nil,           // arguments
	)

	if err != nil {
		l.Error("Error in declaring exchange to consume message")
		return err
	}

	q, err := ch.QueueDeclare(
		fmt.Sprintf("%s_%s", snakeTypeName, "queue"), // name
		false, // durable
		false, // delete when unused
		true,  // exclusive
		false, // no-wait
		nil,   // arguments
	)

	if err != nil {
		l.Error("Error in declaring queue to consume message")
		return err
	}

	err = ch.QueueBind(
		q.Name,        // queue name
		snakeTypeName, // routing key
		snakeTypeName, // exchange
		false,
		nil)
	if err != nil {
		l.Error("Error in binding queue to consume message")
		return err
	}

	deliveries, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto ack
		false,  // exclusive
		false,  // no local
		false,  // no wait
		nil,    // args
	)
	var messageByte []byte
	for d := range deliveries {
		l.Infof(" [x] %s", d.Body)
		messageByte = d.Body
		err = c.handler.Handle(ctx, messageByte, func() error {
			return nil
		})
		if err != nil {
			l.With(zap.Error(err)).Error("Got error handling message")
			return err
		}
	}

	return nil
}
