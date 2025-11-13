.PHONY: build docker-build deploy clean generate-certs

# Build the webhook binary
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o webhook main.go

# Build the Docker image
docker-build:
	docker build -t disk-compatibility-webhook:latest .

# Generate certificates and deploy to Kubernetes
deploy: generate-certs
	kubectl apply -f deploy/namespace.yaml
	kubectl apply -f deploy/rbac.yaml
	kubectl apply -f deploy/service.yaml
	kubectl apply -f deploy/deployment.yaml
	kubectl apply -f deploy/webhook.yaml

# Generate certificates for the webhook
generate-certs:
	./deploy/generate-certs.sh

# Clean up deployment
clean:
	kubectl delete -f deploy/webhook.yaml --ignore-not-found
	kubectl delete -f deploy/deployment.yaml --ignore-not-found
	kubectl delete -f deploy/service.yaml --ignore-not-found
	kubectl delete -f deploy/rbac.yaml --ignore-not-found
	kubectl delete namespace disk-compatibility-webhook --ignore-not-found

# Run tests
test:
	go test -v ./...

# Run linter
lint:
	golangci-lint run
