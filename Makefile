up:
	docker compose up -d

ls:
	go run . topics ls

sub:
	go run . subscribe foo

send-pipe-json-file:
	while read p; do; echo "$p"; done<fixtures/multiple_jsons.txt | go run . send foo

send-pipe-json-single:
	echo '{"hello": "single json world!"}' | go run . send foo

send-arg-json-single:
	go run . send foo '{"hello": "single json world!"}'