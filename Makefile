default: build

schema: app/schema.trust
	cd app && ./compile

consumer:
	go build ./cmd/consumer

producer:
	go build ./cmd/producer

build: schema consumer producer
