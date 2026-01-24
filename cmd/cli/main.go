/*
Package main provides a CLI tool for Kafka administration and debugging.

This tool offers various subcommands for managing topics, inspecting messages,
and performing operational tasks.

Usage:

	kafka-cli <command> [arguments]

Commands:

	topics      Manage Kafka topics (list, create, delete, describe)
	consume     Consume and display messages from a topic
	produce     Produce test messages to a topic
	dlq         Manage Dead Letter Queue (list, replay, purge)
	health      Check Kafka cluster health

Examples:

	kafka-cli topics list
	kafka-cli consume orders --from-beginning
	kafka-cli dlq list --topic orders-dlq
	kafka-cli dlq replay --id <message-id>
*/
package main

import (
	"fmt"
	"os"

	"kafka-demo/pkg/models"
)

const (
	cmdTopics  = "topics"
	cmdConsume = "consume"
	cmdProduce = "produce"
	cmdDLQ     = "dlq"
	cmdHealth  = "health"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case cmdTopics:
		handleTopics(os.Args[2:])
	case cmdConsume:
		handleConsume(os.Args[2:])
	case cmdProduce:
		handleProduce(os.Args[2:])
	case cmdDLQ:
		handleDLQ(os.Args[2:])
	case cmdHealth:
		handleHealth(os.Args[2:])
	case "-h", "--help", "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`kafka-cli - Kafka Administration Tool

Usage:
    kafka-cli <command> [arguments]

Commands:
    topics      Manage Kafka topics (list, create, delete, describe)
    consume     Consume and display messages from a topic
    produce     Produce test messages to a topic
    dlq         Manage Dead Letter Queue (list, replay, purge)
    health      Check Kafka cluster health

Global Options:
    -broker     Kafka broker address (default: localhost:9092)
    -h, --help  Show this help message

Examples:
    kafka-cli topics list
    kafka-cli topics create --name orders --partitions 3 --replication 1
    kafka-cli consume orders --from-beginning --limit 10
    kafka-cli produce orders --count 5 --interval 1s
    kafka-cli dlq list --topic orders-dlq
    kafka-cli dlq replay --id <message-id>
    kafka-cli health`)
}

func handleTopics(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: kafka-cli topics <subcommand>")
		fmt.Println("Subcommands: list, create, delete, describe")
		return
	}

	subcommand := args[0]
	switch subcommand {
	case "list":
		fmt.Printf("Listing topics on %s...\n", models.DefaultKafkaBroker)
		// TODO: Implement topic listing using pkg/kafka
	case "create":
		fmt.Println("Creating topic...")
		// TODO: Implement topic creation
	case "delete":
		fmt.Println("Deleting topic...")
		// TODO: Implement topic deletion
	case "describe":
		fmt.Println("Describing topic...")
		// TODO: Implement topic description
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n", subcommand)
	}
}

func handleConsume(args []string) {
	topic := models.DefaultTopic
	if len(args) > 0 {
		topic = args[0]
	}
	fmt.Printf("Consuming from topic: %s\n", topic)
	// TODO: Implement message consumption with formatting
}

func handleProduce(args []string) {
	topic := models.DefaultTopic
	if len(args) > 0 {
		topic = args[0]
	}
	fmt.Printf("Producing to topic: %s\n", topic)
	// TODO: Implement test message production
}

func handleDLQ(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: kafka-cli dlq <subcommand>")
		fmt.Println("Subcommands: list, replay, purge, stats")
		return
	}

	subcommand := args[0]
	switch subcommand {
	case "list":
		fmt.Printf("Listing DLQ messages from %s...\n", models.DLQTopic)
		// TODO: Implement DLQ message listing
	case "replay":
		fmt.Println("Replaying DLQ message...")
		// TODO: Implement message replay
	case "purge":
		fmt.Println("Purging DLQ...")
		// TODO: Implement DLQ purge
	case "stats":
		fmt.Println("DLQ statistics...")
		// TODO: Implement DLQ statistics
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n", subcommand)
	}
}

func handleHealth(args []string) {
	fmt.Printf("Checking Kafka cluster health at %s...\n", models.DefaultKafkaBroker)
	// TODO: Implement health check
	fmt.Println("Status: OK")
}
