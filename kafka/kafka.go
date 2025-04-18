package kafka

import (
	"context"
	"fmt"
	"github.com/segmentio/kafka-go"
	"log"
	"number-server/models"
	"number-server/utils"
	"strconv"
	"sync"
	"time"
)

type Producer struct {
	writers map[string]*kafka.Writer
	mutex   sync.Mutex
}

func NewProducer(brokers []string) *Producer {
	writers := make(map[string]*kafka.Writer)
	writers["numbers"] = &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    "numbers",
		Balancer: &kafka.LeastBytes{},
	}
	return &Producer{writers: writers}
}

func (p *Producer) Send(ctx context.Context, topic, value string) error {
	p.mutex.Lock()
	writer, ok := p.writers[topic]
	p.mutex.Unlock()
	if !ok {
		return fmt.Errorf("unknown topic: %s", topic)
	}
	return utils.Retry(5, 2*time.Second, func() error {
		return writer.WriteMessages(ctx, kafka.Message{Value: []byte(value)})
	})
}

func (p *Producer) Close() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	for _, writer := range p.writers {
		writer.Close()
	}
}

type Consumer struct {
	logger  *log.Logger
	worker  models.Worker
	readers map[string]*kafka.Reader
	wg      sync.WaitGroup
	closed  chan struct{}
}

func NewConsumer(logger *log.Logger, worker models.Worker, brokers []string, groupID string) *Consumer {
	c := &Consumer{
		logger:  logger,
		worker:  worker,
		readers: make(map[string]*kafka.Reader),
		closed:  make(chan struct{}),
	}
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   "numbers",
		GroupID: groupID,
	})
	c.readers["numbers"] = reader
	c.wg.Add(1)
	go c.consume("numbers", reader)
	return c
}

func (c *Consumer) consume(topic string, reader *kafka.Reader) {
	defer c.wg.Done()
	for {
		select {
		case <-c.closed:
			reader.Close()
			return
		default:
			msg, err := reader.ReadMessage(context.Background())
			if err != nil {
				c.logger.Printf("Error reading from %s: %v", topic, err)
				time.Sleep(2 * time.Second)
				continue
			}
			num, err := strconv.Atoi(string(msg.Value))
			if err != nil {
				c.logger.Printf("Error parsing number from %s: %v", topic, err)
				continue
			}
			c.logger.Printf("Sent number %d to worker from topic %s", num, topic)
		}
	}
}

func (c *Consumer) Close() {
	close(c.closed)
	c.wg.Wait()
	for _, reader := range c.readers {
		reader.Close()
	}
}
