package factories

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
)

var (
	ErrFailedToCreateChannel = fmt.Errorf("unable to create channel")
	ErrFailedToPublish       = fmt.Errorf("unable to publish message")
)

type QueueFactory struct {
	amqpConn  *amqp.Connection
	txChannel *amqp.Channel

	rxChannels sync.Map

	amqpConnectionString string
	tlsConfig            *tls.Config
}

type consumer struct {
	queueName    string
	consumerName string
	handler      Handler
	channel      *amqp.Channel
}

type QueueMessage struct {
	RoutingKey string
	Exchange   string
	Type       string
	Data       []byte
	Mandatory  bool
	ReplyTo    string
	Expiration string
}

func NewQueueInstance(user, pass, host, port string, tlsConfig *tls.Config, options ...func(*QueueFactory)) (*QueueFactory, error) {

	queueInstance := &QueueFactory{
		amqpConnectionString: fmt.Sprintf("amqp://%s:%s@%s:%s", user, pass, host, port),
	}

	for _, option := range options {
		option(queueInstance)
	}
	err := queueInstance.newAMQPConnection(queueInstance.amqpConnectionString, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create amqp connection: %v", err)
	}

	return queueInstance, nil
}

func NewQueueInstanceFromConnectionString(connectionString string, tlsConfig *tls.Config, options ...func(*QueueFactory)) (*QueueFactory, error) {
	queueInstance := &QueueFactory{
		amqpConnectionString: connectionString,
	}

	for _, option := range options {
		option(queueInstance)
	}
	err := queueInstance.newAMQPConnection(queueInstance.amqpConnectionString, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create amqp connection: %v", err)
	}

	return queueInstance, nil
}

func WithTLSConfig(clientCertPath, clientKeyPath, caCertPath string) func(*QueueFactory) {
	return func(qf *QueueFactory) {
		cert, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
		if err != nil {
			panic(err)
		}

		// Try to get system cert pool, but create a new one if it fails (e.g., in Docker)
		certPool, err := x509.SystemCertPool()
		if err != nil || certPool == nil {
			certPool = x509.NewCertPool()
		}

		rootcert, err := os.ReadFile(caCertPath)
		if err != nil {
			panic(err)
		}
		if !certPool.AppendCertsFromPEM(rootcert) {
			panic(fmt.Errorf("failed to append CA certificate from %s", caCertPath))
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      certPool,
		}

		qf.tlsConfig = tlsConfig
		qf.amqpConnectionString = strings.Replace(qf.amqpConnectionString, "amqp://", "amqps://", 1)

	}
}

func (QF *QueueFactory) newAMQPConnection(amqpConnString string, tlsConfig *tls.Config) error {
	// Use stored TLS config if available, otherwise use the parameter
	configToUse := QF.tlsConfig
	if configToUse == nil {
		configToUse = tlsConfig
	}
	
	// Log connection attempt
	if configToUse != nil || strings.HasPrefix(amqpConnString, "amqps://") {
		otelzap.L().Sugar().Infof("connecting to AMQP server with TLS: %v", amqpConnString)
	} else {
		otelzap.L().Sugar().Infof("connecting to AMQP server: %v", amqpConnString)
	}
	
	// Check if connection string is amqps:// or if TLS config is provided
	if configToUse == nil && !strings.HasPrefix(amqpConnString, "amqps://") {
		amqpConnection, err := amqp.Dial(amqpConnString)
		if err != nil {
			return fmt.Errorf("unable to establish amqp connection: %v", err)
		}
		QF.amqpConn = amqpConnection
		otelzap.L().Sugar().Info("successfully connected to AMQP server (plain connection)")
		return nil
	}
	
	// Use TLS connection
	if configToUse == nil {
		// If amqps:// but no TLS config, create a default one
		configToUse = &tls.Config{}
	}
	amqpConnection, err := amqp.DialTLS(amqpConnString, configToUse)
	if err != nil {
		return fmt.Errorf("unable to establish amqp connection: %v", err)
	}
	otelzap.L().Sugar().Info("successfully connected to AMQP server (TLS connection)")
	errChan := amqpConnection.NotifyClose(make(chan *amqp.Error))
	go func() {
		err, open := <-errChan
		if err != nil {
			otelzap.L().Sugar().Errorf("amqp connection closed with error: %v", err)
		}
		otelzap.L().Sugar().Warn("amqp connection closed, attempting to reconnect")
		if !open {
			// attempt to reconnect
			err := QF.newAMQPConnection(amqpConnString, QF.tlsConfig)
			if err != nil {
				otelzap.L().Sugar().Errorf("unable to reconnect to amqp: %v", err)
				return
			}
			// after reconnecting, re-establish all the channels
			QF.rxChannels.Range(
				func(key, value interface{}) bool {
					consumer, ok := value.(consumer)
					if !ok {
						otelzap.L().Sugar().Error("failed to convert value to consumer", "value", value)
						return false
					}
					if err := QF.Consume(context.Background(), consumer.queueName, consumer.consumerName, consumer.handler); err != nil {
						otelzap.L().Sugar().Errorf("unable to re-establish consumer: %v", err)
					}
					return true
				},
			)
		}
	}()
	QF.amqpConn = amqpConnection
	return nil
}

func (factory *QueueFactory) Close() error {
	if factory.amqpConn != nil {
		if factory.txChannel != nil {
			err := factory.txChannel.Close()
			if err != nil {
				return fmt.Errorf("unable to close channel: %v", err)
			}
		}
		return factory.amqpConn.Close()
	}

	factory.rxChannels.Range(
		func(key, value interface{}) bool {
			consumer, ok := value.(consumer)
			if !ok {
				return false
			}
			if err := consumer.channel.Close(); err != nil {
				otelzap.L().Sugar().Errorf("unable to close channel: %v", err)
			}
			return true
		},
	)

	return nil
}

func (factory *QueueFactory) NewTxChannel() (*amqp.Channel, error) {
	var err error
	factory.txChannel, err = factory.amqpConn.Channel()
	if err != nil {
		return nil, errors.Join(ErrFailedToCreateChannel, err)
	}
	return factory.txChannel, nil
}

func (factory *QueueFactory) NewRxChannel() (*amqp.Channel, error) {

	return factory.amqpConn.Channel()
}

type Handler func(amqp.Delivery) (context.Context, error)

func (factory *QueueFactory) Consume(ctx context.Context, queueName string, consumerName string, handler Handler) error {
	channel, err := factory.NewRxChannel()
	if err != nil {
		return err
	}

	factory.rxChannels.Store(queueName, consumer{queueName, consumerName, handler, channel})

	msgs, err := channel.ConsumeWithContext(ctx,
		queueName,
		consumerName,
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	go func() {
		for msg := range msgs {
			ctx, err := handler(msg)
			if err != nil {
				otelzap.L().Sugar().ErrorfContext(ctx, "failed to process message: %v", err)
				if err := channel.Nack(msg.DeliveryTag, false, true); err != nil {
					otelzap.L().Sugar().ErrorfContext(ctx, "unable to nack message: %v", err)
				}
				continue
			}
			if err := msg.Ack(false); err != nil {
				otelzap.L().Sugar().ErrorfContext(ctx, "unable to ack message: %v", err)
			}
		}
	}()
	return nil
}

func (factory *QueueFactory) PublishMessage(message QueueMessage) error {
	if factory.txChannel == nil {
		// create a new channel
		_, err := factory.NewTxChannel()
		if err != nil {
			return err
		}
	}

	var errCount int

publish:
	publishing := amqp.Publishing{
		ContentType: "application/octet-stream",
		Body:        message.Data,
		Type:        string(message.Type),
		ReplyTo:     message.ReplyTo,
	}

	if message.Expiration != "" {
		publishing.Expiration = message.Expiration
	}

	err := factory.txChannel.Publish(
		string(message.Exchange),
		string(message.RoutingKey),
		message.Mandatory,

		// this is not implemented on any versions of rabbitmq after 3.x
		false,
		publishing,
	)
	if err != nil {
		errCount++
		if errors.Is(err, amqp.ErrClosed) {
			if errCount > 2 {
				return ErrFailedToPublish
			}
			// create a new channel
			_, err := factory.NewTxChannel()
			if err != nil {
				return err
			}
			// retry publishing
			goto publish
		}
	}
	return nil
}
