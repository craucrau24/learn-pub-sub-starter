package pubsub

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type SimpleQueueType string 
const (
	DurableQueueType SimpleQueueType = "durable"
	TransientQueueType SimpleQueueType = "transient"

)

type AckType int 
const (
	Ack AckType = iota
	NackRequeue
	NackDiscard
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	jsonData, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("error while marshalling val to JSON: %w", err)
	}
	return ch.PublishWithContext(context.Background(), exchange, key, false, false, amqp.Publishing{ContentType: "application/json", Body: jsonData})
}

func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType SimpleQueueType, // an enum to represent "durable" or "transient"
) (*amqp.Channel, amqp.Queue, error) {
		channel, err := conn.Channel()
		if err != nil {
			return nil, amqp.Queue{}, fmt.Errorf("error during channel creation: %w", err)
		}

		queue, err := channel.QueueDeclare(queueName, queueType == "durable", queueType == "transient", queueType == "transient", false, amqp.Table{"x-dead-letter-exchange": "peril_dlx"})
		if err != nil {
			return nil, amqp.Queue{}, fmt.Errorf("error during queue declaration: %w", err)
		}
		
		err = channel.QueueBind(queue.Name, key, exchange, false, nil)
		if err != nil {
			return nil, amqp.Queue{}, fmt.Errorf("error during queue binding: %w", err)
		}

		return channel, queue, nil
}

func SubscribeJSON[T any](
    conn *amqp.Connection,
    exchange,
    queueName,
    key string,
    queueType SimpleQueueType, // an enum to represent "durable" or "transient"
    handler func(T) AckType,
) error {
	channel, _, err := DeclareAndBind(conn, exchange, queueName, key, queueType)
	if err != nil {
		return fmt.Errorf("error while binding queue: %w", err)
	}
	deliveries, err := channel.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("error while consuming queue: %w", err)
	}

	go func() {
		for d := range deliveries {
			var value T
			err := json.Unmarshal(d.Body, &value)
			if err == nil {
				switch handler(value) {
				case Ack:
					d.Ack(false)
					fmt.Println("Message acknowledged")
					continue
				case NackRequeue:
					d.Nack(false, true)
					fmt.Println("Message requeued")
					continue
				case NackDiscard:
					d.Nack(false, false)
					fmt.Println("Message discarded")
					continue
				}
			}
			d.Nack(false, false)
			fmt.Println("Error decoding message")
		}
	}()

	return nil
}