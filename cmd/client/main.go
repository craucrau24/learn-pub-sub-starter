package main

import (
	"fmt"
	"log"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.AckType {
	return func(state routing.PlayingState) pubsub.AckType {
		defer fmt.Print("> ")
		gs.HandlePause(state)
		return pubsub.Ack
	}
}

func handlerMove(gs *gamelogic.GameState, conn *amqp.Connection) func(gamelogic.ArmyMove) pubsub.AckType {
	channel, err := conn.Channel()
	if err != nil {
		log.Fatalf("error while creating channel in handlerMove(): %v\n", err)
	}

	return func(move gamelogic.ArmyMove) pubsub.AckType {
		defer fmt.Print("> ")
		outcome := gs.HandleMove(move)
		switch outcome {
		case gamelogic.MoveOutComeSafe:
			return pubsub.Ack
		case gamelogic.MoveOutcomeMakeWar:
			err := pubsub.PublishJSON(channel, routing.ExchangePerilTopic, fmt.Sprintf("%s.%s", routing.WarRecognitionsPrefix, gs.GetUsername()), gamelogic.RecognitionOfWar{Attacker: move.Player, Defender: gs.GetPlayerSnap()})
			if err == nil {
				return pubsub.Ack
			} else {
				return pubsub.NackRequeue
			}
		case gamelogic.MoveOutcomeSamePlayer:
			return pubsub.NackDiscard
		default:
			return pubsub.NackDiscard
		}
	}
}

func handlerWar(gs *gamelogic.GameState) func(gamelogic.RecognitionOfWar) pubsub.AckType {
	return func(row gamelogic.RecognitionOfWar) pubsub.AckType {
		defer fmt.Println("> ")

		outcome, _, _ := gs.HandleWar(row)
		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon:
			return pubsub.Ack
		case gamelogic.WarOutcomeYouWon:
			return pubsub.Ack
		case gamelogic.WarOutcomeDraw:
			return pubsub.Ack
		default:
			fmt.Println("Unknown war outcome in war handler")
			return pubsub.NackDiscard
		}
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
		log.Fatalf("Couldn't create subscribe to queue `%s`: %v\n", fmt.Sprintf("%s.%s", routing.PauseKey, name), err)
	}
	err = pubsub.SubscribeJSON(conn, string(routing.ExchangePerilTopic), fmt.Sprintf("%s.%s", routing.ArmyMovesPrefix, name), fmt.Sprintf("%s.*", routing.ArmyMovesPrefix), pubsub.TransientQueueType, handlerMove(state, conn))
	if err != nil {
		log.Fatalf("Couldn't create subscribe to queue `%s`: %v\n", fmt.Sprintf("%s.%s", routing.ArmyMovesPrefix, name), err)
	}

	err = pubsub.SubscribeJSON(conn, string(routing.ExchangePerilTopic), "war", fmt.Sprintf("%s.*", routing.WarRecognitionsPrefix), pubsub.DurableQueueType, handlerWar(state))
	if err != nil {
		log.Fatalf("Couldn't create subscribe to queue `%s`: %v\n", "war", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		log.Fatalf("Couldn't open channel for publishing%v\n", err)
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
			move, err := state.CommandMove(words)
			if err != nil {
				fmt.Printf("Couldn't move unit(s): %v\n", err)
				break out
			}
			err = pubsub.PublishJSON(channel, string(routing.ExchangePerilTopic), fmt.Sprintf("%s.%s", routing.ArmyMovesPrefix, name), move)
			if err != nil {
				fmt.Printf("Couldn't publish move: %v\n", err)
			} else {
				fmt.Println("Move published successfully")
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
