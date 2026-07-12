SHELL := /bin/bash

#default: fmt lint install generate

build:
	go build -v ./...

# install: build
# 	go install -v ./...

install:
	go install .

lint:
	act -j lint
#golangci-lint run

generate:
	cd tools; go generate ./...

fmt:
	gofmt -s -w -e .

test:
	TF_ACC=1 go test ./... -count=1  -v -cover -timeout=120s -parallel=10

testacc:
	TF_ACC=1 go test -v -cover -timeout 120m ./...

up:
	docker-compose up -d

down:
	docker-compose stop
	docker-compose rm -f

pretest: down up

apply:
	TF_LOG_PROVIDER=INFO terraform apply --auto-approve

debug:
	TF_LOG_PROVIDER=DEBUG terraform apply --auto-approve

plan:
	TF_LOG_PROVIDER=DEBUG terraform plan

docs:
	cd tools && go generate ./... && cd ..

clean:
	@rm -rf terraform.tfstate terraform.tfstate.*

.PHONY: fmt lint test testacc install generate apply plan docs

.SILENT: docs
