.PHONY: test vet images console

console:
	cd console && npm install && npm run build

test:
	go test ./...

vet:
	go vet ./...

images:
	docker build -t pulse-ingest:local --build-arg CMD=ingest .
	docker build -t pulse-processor:local --build-arg CMD=processor .
	docker build -t pulse-query:local --build-arg CMD=query .
	docker build -t pulse-graphql:local --build-arg CMD=graphql .
