package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/manzanit0/beetlectl/kafka"
	"github.com/olekukonko/tablewriter"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "beetlectl",
		Usage: "making (somewhat) easier to interact with Kafka",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "brokers",
				Required: false,
				Value:    "localhost:9092",
				Usage:    "broker seeds for initial discovery",
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "subscribe",
				Usage: "subscribe to a topic",
				Action: func(ctx *cli.Context) error {
					client, err := clientFromCtx(ctx)
					if err != nil {
						return err
					}
					defer client.Close()

					fmt.Println("This command will start streaming all messages. To exit, press CTRL+C once.")
					err = client.Subscribe(ctx.Context, ctx.Args().First())
					if err != nil {
						return err
					}

					return nil
				},
			},
			{
				Name:  "send",
				Usage: "Send a message to topic",
				Action: func(ctx *cli.Context) error {
					client, err := clientFromCtx(ctx)
					if err != nil {
						return err
					}
					defer client.Close()

					// Support passing the message as an argument
					data := []byte(ctx.Args().Get(1))
					if len(data) > 0 {
						err = client.Send(ctx.Context, ctx.Args().First(), data)
						if err != nil {
							return err
						}

						return nil
					}

					// Support streaming the messages through STDIN.
					reader := bufio.NewReader(os.Stdin)
					for {
						line, _, err := reader.ReadLine()
						if errors.Is(err, io.EOF) {
							return nil
						} else if err != nil {
							return fmt.Errorf("reading line from STDIN: %w", err)
						}

						log.Println("sending line")
						err = client.Send(ctx.Context, ctx.Args().First(), line)
						if err != nil {
							return fmt.Errorf("send message: %w", err)
						}
					}
				},
			},
			{
				Name: "groups",
				Subcommands: []*cli.Command{
					{
						Name:        "ls",
						Usage:       "list groups",
						Description: "list groups",
						Action: func(ctx *cli.Context) error {
							client, err := clientFromCtx(ctx)
							if err != nil {
								return err
							}
							defer client.Close()

							groups, err := client.ListGroups(ctx.Context)
							if err != nil {
								return err
							}

							if len(groups) == 0 {
								fmt.Println("There are no groups!")
								return nil
							}

							table := tablewriter.NewWriter(os.Stdout)
							table.SetHeader([]string{"Name", "State", "Protocol"})
							for _, g := range groups {
								table.Append([]string{g.Group, g.GroupState, g.ProtocolType})
							}

							table.Render()

							return nil
						},
					},
					{
						Name:        "lag",
						Usage:       "check lag for a given consumer",
						Description: "check lag for a given consumer",
						Args:        true,
						Action: func(ctx *cli.Context) error {
							client, err := clientFromCtx(ctx)
							if err != nil {
								return err
							}
							defer client.Close()

							memberLags, err := client.CalculateLag(ctx.Context, ctx.Args().First())
							if err != nil {
								return err
							}

							table := tablewriter.NewWriter(os.Stdout)
							table.SetHeader([]string{"Topic", "Partition", "Member ID","Partition Offset","Member Offset", "Lag"})
							for _, lag := range memberLags {
								table.Append([]string{
									lag.Topic,
									fmt.Sprint(lag.Partition),
									lag.Member.MemberID,
									fmt.Sprint(lag.End.Offset),
									fmt.Sprint(lag.Commit.At),
									fmt.Sprint(lag.Lag),
								})
							}

							table.Render()
							return nil
						},
					},
				},
			},
			{
				Name:        "topics",
				Usage:       "interact with kafka topics",
				Description: "interact with kafka topics",
				Subcommands: []*cli.Command{
					{
						Name:    "list",
						Usage:   "list all available topics",
						Aliases: []string{"ls"},
						Action: func(ctx *cli.Context) error {
							client, err := clientFromCtx(ctx)
							if err != nil {
								return err
							}
							defer client.Close()

							topics, err := client.ListTopics(ctx.Context)
							if err != nil {
								return err
							}

							if len(topics) == 0 {
								fmt.Println("There are no topics!")
								return nil
							}

							table := tablewriter.NewWriter(os.Stdout)
							table.SetHeader([]string{"Name", "Partitions", "Retention Mins", "Max Message Bytes", "Replication"})
							for _, t := range topics {
								var retentionMillis, maxMessageBytes, replication string
								for _, c := range t.Configs {
									switch {
									case c.Key == "retention.ms":
										retentionMillis = *c.Value
									case c.Key == "max.message.bytes":
										maxMessageBytes = *c.Value
									case c.Key == "min.insync.replicas":
										replication = *c.Value
									}
								}
								duration, err := time.ParseDuration(retentionMillis + "ms")
								if err != nil {
									return err
								}
								table.Append([]string{
									*t.Topic.Topic,
									fmt.Sprint(len(t.Topic.Partitions)),
									fmt.Sprint(duration.Minutes()),
									maxMessageBytes,
									replication,
								})
							}

							table.Render()
							return nil
						},
					},
					{
						Name:  "create",
						Usage: "create a topic",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "name",
								Required: true,
								Aliases:  []string{"n"},
								Usage:    "name of the topic",
							},
							&cli.StringFlag{
								Name:     "partitions",
								Required: false,
								Value:    "1",
								Aliases:  []string{"p"},
								Usage:    "amount of partitions for the topic",
							},
						},
						Action: func(ctx *cli.Context) error {
							client, err := clientFromCtx(ctx)
							if err != nil {
								return err
							}
							defer client.Close()

							name := ctx.String("name")
							partitions, err := strconv.ParseInt(ctx.String("partitions"), 10, 32)
							if err != nil {
								return fmt.Errorf("partitions must be an integer!")
							}

							err = client.CreateTopic(ctx.Context, name, int32(partitions))
							if err != nil {
								return err
							}

							fmt.Println("Topic", name, "created!")
							return nil
						},
					},
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func clientFromCtx(ctx *cli.Context) (kafka.Client, error) {
	brokers := strings.Split(ctx.String("brokers"), ",")
	client := kafka.New(kgo.SeedBrokers(brokers...))

	err := client.Ping(ctx.Context)
	if err != nil {
		return nil, err
	}
	return client, nil
}
