package kafka

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

const (
	kafkaHeaderFormat     = "format"
	kafkaHeaderFormatJSON = "json"
)

type Client interface {
	Ping(ctx context.Context) error
	CreateTopic(ctx context.Context, topic string, partitions int32) error
	ListTopics(ctx context.Context) ([]TopicDescription, error)
	CalculateLag(ctx context.Context, groupID string) ([]kadm.GroupMemberLag, error)
	ListGroups(ctx context.Context) ([]kmsg.ListGroupsResponseGroup, error)
	Subscribe(ctx context.Context, topic string) error
	Send(ctx context.Context, topic string, data []byte) error
	Close()
}

type client struct {
	opts []kgo.Opt
	*kgo.Client
}

var _ Client = (*client)(nil)

func New(opts ...kgo.Opt) *client {
	cl, err := kgo.NewClient(opts...)
	if err != nil {
		panic(err)
	}

	return &client{
		opts:   opts,
		Client: cl,
	}
}

func (c *client) Ping(ctx context.Context) error {
	return c.Client.Ping(ctx)
}

func (c *client) CreateTopic(ctx context.Context, topic string, partitions int32) error {
	createTopicReq := kmsg.NewCreateTopicsRequest()
	createTopicReq.Topics = append(createTopicReq.Topics, kmsg.CreateTopicsRequestTopic{Topic: topic, NumPartitions: partitions, ReplicationFactor: 1})

	res, err := createTopicReq.RequestWith(ctx, c.Client)
	if err != nil {
		return fmt.Errorf("create topic: %w", err)
	}

	for i := range res.Topics {
		if res.Topics[i].ErrorMessage != nil {
			err = errors.Join(err, fmt.Errorf("create topic: %s", *res.Topics[i].ErrorMessage))
		}
	}

	return err
}

type TopicDescription struct {
	Topic   kmsg.MetadataResponseTopic
	Configs []kadm.Config
}

func (c *client) ListTopics(ctx context.Context) ([]TopicDescription, error) {
	req := kmsg.NewMetadataRequest()
	res, err := req.RequestWith(ctx, c.Client)
	if err != nil {
		return nil, fmt.Errorf("request topics: %w", err)
	}

	// TODO: there can be some error handling here with requesting topics, ie. authz

	var topics []string
	for _, t := range res.Topics {
		topics = append(topics, *t.Topic)
	}

	admin := kadm.NewClient(c.Client)
	defer admin.Close()

	var descriptions []TopicDescription
	dd, _ := admin.DescribeTopicConfigs(ctx, topics...)
	for _, t := range res.Topics {
		config, err := dd.On(*t.Topic, func(rc *kadm.ResourceConfig) error { return nil })
		if err != nil {
			return nil, fmt.Errorf("get resource config: %w", err)
		}

		descriptions = append(descriptions, TopicDescription{
			Topic:   t,
			Configs: config.Configs,
		})
	}

	return descriptions, nil
}

func (c *client) ListGroups(ctx context.Context) ([]kmsg.ListGroupsResponseGroup, error) {
	req := kmsg.NewListGroupsRequest()
	res, err := req.RequestWith(ctx, c.Client)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}

	return res.Groups, nil
}

func (c *client) Subscribe(ctx context.Context, topic string) error {
	// We create a new client to be able to specify the consumer group and
	// topics to consume.
	opts := append(c.opts, kgo.ConsumerGroup("beetlectl"), kgo.ConsumeTopics(topic), kgo.DisableAutoCommit())
	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return fmt.Errorf("new client: %w", err)
	}
	defer cl.Close()

	for {
		fetches := cl.PollFetches(ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			// All errors are retried internally when fetching, but non-retriable errors are
			// returned from polls so that users can notice and take action.
			return fmt.Errorf(fmt.Sprint(errs))
		}

		fetches.EachPartition(func(p kgo.FetchTopicPartition) {
			for _, record := range p.Records {
				defer func(r *kgo.Record) {
					err = cl.CommitRecords(ctx, r)
					if err != nil {
						fmt.Println("error", err.Error())
					}
					fmt.Println("Committed record")
				}(record)

				for _, h := range record.Headers {
					if h.Key == kafkaHeaderFormat && string(h.Value) == kafkaHeaderFormatJSON {
						var prettyJSON bytes.Buffer
						err = json.Indent(&prettyJSON, record.Value, "", " ")
						if err != nil {
							fmt.Printf("indenting JSON: %s\n", err.Error())
						}

						b, err := io.ReadAll(&prettyJSON)
						if err != nil {
							fmt.Printf("reading indented JSON: %s\n", err.Error())
						}

						fmt.Println(string(b))
						return
					}
				}

				fmt.Println(string(record.Value))
			}
		})
	}
}

func (c *client) Send(ctx context.Context, topic string, data []byte) error {
	// We create a new client to be able to specify the consumer group and
	// topics to consume.
	opts := append(c.opts, kgo.ConsumerGroup("beetlectl"), kgo.ConsumeTopics(topic))
	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return fmt.Errorf("new client: %w", err)
	}
	defer cl.Close()

	record := &kgo.Record{Topic: topic, Value: data}

	if IsJSON(string(data)) {
		record.Headers = append(record.Headers, kgo.RecordHeader{
			Key:   kafkaHeaderFormat,
			Value: []byte(kafkaHeaderFormatJSON),
		})
	}

	if err := cl.ProduceSync(ctx, record).FirstErr(); err != nil {
		return fmt.Errorf("produce sync: %w", err)
	}

	return nil
}

func IsJSON(str string) bool {
	var m json.RawMessage
	return json.Unmarshal([]byte(str), &m) == nil
}

func (c *client) CalculateLag(ctx context.Context, consumerGroupID string) ([]kadm.GroupMemberLag, error) {
	admin := kadm.NewClient(c.Client)
	defer admin.Close()

	consumerGroupID = strings.TrimSpace(consumerGroupID)
	g, err := admin.Lag(ctx, consumerGroupID)
	if err != nil {
		return nil, fmt.Errorf("lag: %w", err)
	} else if g.Error() != nil {
		return nil, fmt.Errorf("lag: %w", err)
	}

	if len(g.Sorted()) > 1 {
		return nil, fmt.Errorf("got more than one group: %w", err)
	}

	group := g.Sorted()[0]
	if group.State != "Stable" {
		return nil, fmt.Errorf("group is %s", group.State)
	}

	return group.Lag.Sorted(), nil
}

func (c *client) Close() {
	c.Client.Close()
}
