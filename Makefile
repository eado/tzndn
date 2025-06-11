default: build

schema: 
	cd app && ./compile

consumer:
	go build -o ./bin/consumer/consumer ./cmd/consumer

producer:
	go build -o ./bin/producer/producer ./cmd/producer

build: consumer producer
