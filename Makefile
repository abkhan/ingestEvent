.PHONY: build run test clean integration-test

build:
	go build -o bin/fulcrum .

run:
	go run .

test:
	go test -v ./... -run "TestValidate|TestHandle|TestIs|TestFlush"

integration-test:
	docker-compose up -d postgres fulcrum
	sleep 10
	docker-compose run --rm test
	docker-compose down

clean:
	rm -rf bin/
	docker-compose down -v