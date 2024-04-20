package main

import (
	"fmt"
	"os"
	"os/signal"
	models "producer-schema/out"
	"syscall"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/confluentinc/confluent-kafka-go/schemaregistry"
	"github.com/confluentinc/confluent-kafka-go/schemaregistry/serde"
	"github.com/confluentinc/confluent-kafka-go/schemaregistry/serde/avro"
)

type Client struct {
	Name    string
	Country string
	City    string
}

var (
	bootstrapServers  = "localhost:9092"
	schemaRegistryURL = "http://localhost:8081"
	topic             = "client"
)

func main() {
	produce()
	consume()
}

func produce() {

	producer, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": bootstrapServers})
	if err != nil {
		panic(err)
	}

	sc, err := schemaregistry.NewClient(schemaregistry.NewConfig(schemaRegistryURL))
	if err != nil {
		panic(err)
	}

	ser, err := avro.NewGenericSerializer(sc, serde.ValueSerde, avro.NewSerializerConfig())
	if err != nil {
		panic(err)
	}

	payload, err := ser.Serialize(topic, &Client{Name: "Ronaldo", Country: "Brazil", City: "SaoPaulo"})
	if err != nil {
		panic(err)
	}

	err = producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          payload,
	}, nil)
	if err != nil {
		panic(err)
	}

	producer.Flush(1000)
}

func consume() {
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":  bootstrapServers,
		"group.id":           "consumer-sc",
		"enable.auto.commit": false,
		"auto.offset.reset":  "earliest"})

	if err != nil {
		panic(err)
	}

	client, err := schemaregistry.NewClient(schemaregistry.NewConfig(schemaRegistryURL))

	if err != nil {
		panic(err)
	}

	deser, err := avro.NewGenericDeserializer(client, serde.ValueSerde, avro.NewDeserializerConfig())

	if err != nil {
		panic(err)
	}

	err = c.SubscribeTopics([]string{topic}, nil)
	if err != nil {
		panic(err)
	}

	run := true

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	for run {
		select {
		case sig := <-sigchan:
			fmt.Printf("Caught signal %v: terminating\n", sig)
			run = false
		default:
			ev := c.Poll(100)
			if ev == nil {
				continue
			}

			switch e := ev.(type) {
			case *kafka.Message:
				value := models.Client{}
				err := deser.DeserializeInto(*e.TopicPartition.Topic, e.Value, &value)
				if err != nil {
					fmt.Printf("Failed to deserialize payload: %s\n", err)
				} else {
					fmt.Printf("%% Message on %s:\n%+v\n", e.TopicPartition, value)
				}
				if e.Headers != nil {
					fmt.Printf("%% Headers: %v\n", e.Headers)
				}
				c.Commit()
			case kafka.Error:
				// Errors should generally be considered
				// informational, the client will try to
				// automatically recover.
				fmt.Fprintf(os.Stderr, "%% Error: %v: %v\n", e.Code(), e)
			default:
				fmt.Printf("Ignored %v\n", e)
			}
		}
	}

}
