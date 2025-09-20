package main

import (
	"fmt"
	"log"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	fmt.Println("Starting Peril server...")
	cxnStr := "amqp://guest:guest@localhost:5672/"

	fmt.Println("RabbitMQ connection attempt...")
	conn, err := amqp.Dial(cxnStr)
	if err != nil {
		log.Fatalf("Couldn't connect to rabbitMQ server: %v\n", err)
	}
	defer conn.Close()
	fmt.Println("RabbitMQ connection successful.")

	channel, err := conn.Channel()
	if err != nil {
		log.Fatalf("Couldn't create channel: %v\n", err)
	}
	
	// channel, _, err := pubsub.DeclareAndBind(conn, routing.ExchangePerilTopic, routing.GameLogSlug, fmt.Sprintf("%s.*", routing.GameLogSlug), pubsub.DurableQueueType)
	// if err != nil {
	// 	log.Fatalf("Couldn't create %s queue: %v\n", routing.GameLogSlug, err)
	// }

	err = pubsub.SubscribeGob(conn, routing.ExchangePerilTopic, routing.GameLogSlug, fmt.Sprintf("%s.*", routing.GameLogSlug), pubsub.DurableQueueType,
	 func(log routing.GameLog) pubsub.AckType {
		defer fmt.Print("> ")
		gamelogic.WriteLog(log)
		return pubsub.Ack
	 })

	if err != nil {
		log.Fatalf("Couldn't subscribe to game_logs key: %v\n", err)
	}

	gamelogic.PrintServerHelp()
	out:
	for {
		words := gamelogic.GetInput()
		if len(words) == 0 {
			continue
		}
		var err error

		switch words[0] {
		case "pause":  {
			fmt.Println("Pause the game!")
			err = pubsub.PublishJSON(channel, string(routing.ExchangePerilDirect), string(routing.PauseKey), routing.PlayingState{IsPaused: true})
		}
		case "resume": {
			fmt.Println("Resume the game!")
			err = pubsub.PublishJSON(channel, string(routing.ExchangePerilDirect), string(routing.PauseKey), routing.PlayingState{IsPaused: false})
		}
		case "quit": {
			break out
		}
		default: {

		}
		}
		if err != nil {
			fmt.Printf("Something went wrong when publishing message: %v", err)
		}
	}

	fmt.Println("Server is stopping...")

}
