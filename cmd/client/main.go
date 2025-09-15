package main

import (
	"fmt"
	"log"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) {
	return func(state routing.PlayingState) {
		defer fmt.Print("> ")
		gs.HandlePause(state)
	}
}

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
	
	state := gamelogic.NewGameState(name)
	err = pubsub.SubscribeJSON(conn, routing.ExchangePerilDirect, fmt.Sprintf("%s.%s", routing.PauseKey, name), routing.PauseKey, pubsub.TransientQueueType, handlerPause(state))
	if err != nil {
		log.Fatalf("Couldn't create subscribe to queue: %v\n", err)
	}


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
			_, err := state.CommandMove(words)
			if err != nil {
				fmt.Printf("Couldn't move unit(s): %v\n", err)
			}
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
