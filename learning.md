## Learning Notes

These are just some notes I've taken as I've learnt new concepts.

## Partitions

- Kafka **guarantees the order of the events within the same topic partition**.
  However, by default, it does not guarantee the order of events across all
  partitions.
- Events with the same event key are assigned to the same partition, which is
  ordered
- If a message has no key, subsequent messages will be distributed round-robin
  among all the topic’s partitions.

## Broker discovery

- Seeds are only used for the initial discovery, the metadata response returns
  all broker-configured advertised listeners -- and the advertised listeners
  are used thereafter by the client.
  ([ref](https://github.com/twmb/franz-go/issues/611))

## Listeners

- When configuring Kafka locally with Docker, getting networking right is critical,
  otherwise clients won't be able to connect. Depending on if you expect clients to
  connect in from the same newtwork or not, you need to advertise listeners one way
  or another.

ref: https://www.confluent.io/blog/kafka-listeners-explained/

## Consumers

- When starting to subscribe to a topic, Kafka triggers a rebalance. This causes
  the consumer to not get any messages for over a minute.
