package pubsub

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type simpleQueueType string 
const (
	DurableQueueType simpleQueueType = "durable"
	TransientQueueType simpleQueueType = "transient"

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
	queueType simpleQueueType, // an enum to represent "durable" or "transient"
) (*amqp.Channel, amqp.Queue, error) {
		channel, err := conn.Channel()
		if err != nil {
			return nil, amqp.Queue{}, fmt.Errorf("error during channel creation: %w", err)
		}

		queue, err := channel.QueueDeclare(queueName, queueType == "durable", queueType == "transient", queueType == "transient", false, nil)
		if err != nil {
			return nil, amqp.Queue{}, fmt.Errorf("error during queue declaration: %w", err)
		}
		
		err = channel.QueueBind(queue.Name, key, exchange, false, nil)
		if err != nil {
			return nil, amqp.Queue{}, fmt.Errorf("error during queue binding: %w", err)
		}

		return channel, queue, nil
}