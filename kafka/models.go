package kafka

type TopicConfig struct {
	Name string `json:"name"`
}

type Config struct {
	// ...
	KafkaTopics []TopicConfig `json:"kafka_topics"`
}
