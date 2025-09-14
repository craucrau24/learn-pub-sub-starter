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
	fmt.Println("Starting Peril client...")
	connStr := "amqp://guest:guest@localhost:5672/"

	fmt.Println("RabbitMQ connection attempt...")
	conn, err := amqp.Dial(connStr)
	if err != nil {
		log.Fatalf("Couldn't connect to rabbitMQ server: %v\n", err)
	}
	defer conn.Close()
	fmt.Println("RabbitMQ connection successful.")

	name, err := gamelogic.ClientWelcome()
	if err != nil {
		log.Fatalf("Unexpected error with name retrieval: %v\n", err)
	}
	
	_, _, err = pubsub.DeclareAndBind(conn, routing.ExchangePerilDirect, fmt.Sprintf("%s.%s", routing.PauseKey, name), routing.PauseKey, pubsub.TransientQueueType)
	if err != nil {
		log.Fatalf("Couldn't create channel and queue: %v\n", err)
	}

	state := gamelogic.NewGameState(name)

	out:
	for {
		words := gamelogic.GetInput()
		if len(words) == 0 {
			continue
		}

		switch words[0] {
		case "spawn":  {
			state.CommandSpawn(words)
		}
		case "move": {
			state.CommandMove(words)
		}
		case "status": {
			state.CommandStatus()
		}
		case "help": {
			gamelogic.PrintClientHelp()
		}
		case "spam": {
			fmt.Println("Spamming not allowed yet!")
		}
		case "quit": {
			gamelogic.PrintQuit()
			break out
		}
		default: {
			fmt.Println("Unknown command")
		}
		}
	}
}
