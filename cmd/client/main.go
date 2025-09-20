package main

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func publishLog(channel *amqp.Channel, message, username string) error {
	log := routing.GameLog{Username: username, Message: message, CurrentTime: time.Now()}
	return pubsub.PublishGob(channel, routing.ExchangePerilTopic, fmt.Sprintf("%s.%s", routing.GameLogSlug, username), log)
}

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.AckType {
	return func(state routing.PlayingState) pubsub.AckType {
		defer fmt.Print("> ")
		gs.HandlePause(state)
		return pubsub.Ack
	}
}

func handlerMove(gs *gamelogic.GameState, channel *amqp.Channel) func(gamelogic.ArmyMove) pubsub.AckType {
	// channel, err := conn.Channel()
	// if err != nil {
	// 	log.Fatalf("error while creating channel in handlerMove(): %v\n", err)
	// }

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

func handlerWar(gs *gamelogic.GameState, channel *amqp.Channel) func(gamelogic.RecognitionOfWar) pubsub.AckType {
	return func(row gamelogic.RecognitionOfWar) pubsub.AckType {
		defer fmt.Print("> ")

		outcome, winner, loser := gs.HandleWar(row)
		username := row.Attacker.Username
		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon:
			err := publishLog(channel, fmt.Sprintf("%s won a war against %s", winner, loser), username)
			if err != nil {
				return pubsub.NackRequeue
			} else {
				return pubsub.Ack
			}
		case gamelogic.WarOutcomeYouWon:
			err := publishLog(channel, fmt.Sprintf("%s won a war against %s", winner, loser), username)
			if err != nil {
				return pubsub.NackRequeue
			} else {
				return pubsub.Ack
			}
		case gamelogic.WarOutcomeDraw:
			err := publishLog(channel, fmt.Sprintf("A war between %s and %s resulted in a draw", winner, loser), username)
			if err != nil {
				return pubsub.NackRequeue
			} else {
				return pubsub.Ack
			}
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
	
	channel, err := conn.Channel()
	if err != nil {
		log.Fatalf("Couldn't open channel for publishing%v\n", err)
	}

	state := gamelogic.NewGameState(name)
	err = pubsub.SubscribeJSON(conn, routing.ExchangePerilDirect, fmt.Sprintf("%s.%s", routing.PauseKey, name), routing.PauseKey, pubsub.TransientQueueType, handlerPause(state))
	if err != nil {
		log.Fatalf("Couldn't create subscribe to queue `%s`: %v\n", fmt.Sprintf("%s.%s", routing.PauseKey, name), err)
	}
	err = pubsub.SubscribeJSON(conn, string(routing.ExchangePerilTopic), fmt.Sprintf("%s.%s", routing.ArmyMovesPrefix, name), fmt.Sprintf("%s.*", routing.ArmyMovesPrefix), pubsub.TransientQueueType, handlerMove(state, channel))
	if err != nil {
		log.Fatalf("Couldn't create subscribe to queue `%s`: %v\n", fmt.Sprintf("%s.%s", routing.ArmyMovesPrefix, name), err)
	}

	err = pubsub.SubscribeJSON(conn, string(routing.ExchangePerilTopic), "war", fmt.Sprintf("%s.*", routing.WarRecognitionsPrefix), pubsub.DurableQueueType, handlerWar(state, channel))
	if err != nil {
		log.Fatalf("Couldn't create subscribe to queue `%s`: %v\n", "war", err)
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
			if len(words) == 2 {
				n, err := strconv.Atoi(words[1])
				if err != nil {
					fmt.Printf("Error `%s` is not an integer", words[1])
					break
				}
				for range n {
					msg := gamelogic.GetMaliciousLog()
					_ = publishLog(channel, msg, name)
				}
			}
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
