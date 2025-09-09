package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	fmt.Println("Starting Peril server...")
	cxnStr := "amqp://guest:guest@localhost:5672/"

	fmt.Println("RabbitMQ connection attempt...")
	cxn, err := amqp.Dial(cxnStr)
	if err != nil {
		log.Fatalf("Couldn't connect to rabbitMQ server: %v", err)
	}
	defer cxn.Close()
	fmt.Println("RabbitMQ connection successful.")

	// wait for ctrl+c
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan
	fmt.Println("Interruption signal received. Closing connection.")

}
