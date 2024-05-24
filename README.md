# beetlectl

CLI to interact with kafka.

Don't expect anything nice. This just a [breakable toy](toy).

## Usage

Some example usages:

### Listing topics groups
```sh
beetlectl topics ls
+--------------------+------------+----------------+-------------------+-------------+
|        NAME        | PARTITIONS | RETENTION MINS | MAX MESSAGE BYTES | REPLICATION |
+--------------------+------------+----------------+-------------------+-------------+
| foo                |          1 |          10080 |           1048588 |           1 |
| __consumer_offsets |         50 |          10080 |           1048588 |           1 |
+--------------------+------------+----------------+-------------------+-------------+
```

### Listing consumer groups

```sh
beetlectl groups ls
+-----------+--------+----------+
|   NAME    | STATE  | PROTOCOL |
+-----------+--------+----------+
| beetlectl | Stable | consumer |
+-----------+--------+----------+
```

### Listing lag for a given consumer group

```sh
beetlectl groups lag beetlectl
+-------+-----------+------------------------------------------+------------------+---------------+-----+
| TOPIC | PARTITION |                MEMBER ID                 | PARTITION OFFSET | MEMBER OFFSET | LAG |
+-------+-----------+------------------------------------------+------------------+---------------+-----+
| foo   |         0 | kgo-55c902e2-bcac-4911-961f-0e51fb37986b |               83 |            83 |   0 |
+-------+-----------+------------------------------------------+------------------+---------------+-----+
```

[toy]: https://walterteng.com/apprenticeship-patterns#breakable-toys