export GO111MODULE := on
export CGO_ENABLED := 0

rwildcard=$(wildcard $1$2) $(foreach d,$(wildcard $1*),$(call rwildcard,$d/,$2))

BIN_DIR ?= bin

PROTO_SRCS += $(wildcard proto/controller/v1/*.proto)

PROTO_OUTPUT += proto/gen/controller/v1/controller.pb.go
PROTO_OUTPUT += proto/gen/controller/v1/controller_grpc.pb.go
# PROTO_OUTPUT += proto/gen/controller/v1/peer.pb.go
# PROTO_OUTPUT += proto/gen/controller/v1/gateway.pb.go
# PROTO_OUTPUT += proto/gen/controller/v1/gateway.pb.gw.go
# PROTO_OUTPUT += proto/gen/controller/v1/gateway_grpc.pb.go

all: controller

tidy:
	@go mod tidy

frontend:
	@docker-compose up --build

docker-controller:
	@docker-compose build controller
	@docker-compose up controller

controller: buf
	go build -o $(BIN_DIR)/controller cmd/controller/main.go

run-controller: controller
	$(BIN_DIR)/controller

node: buf
	go build -o $(BIN_DIR)/node cmd/node/main.go

buf: $(PROTO_OUTPUT)

$(PROTO_OUTPUT): $(PROTO_SRCS)
	@echo Generating proto...
	@buf generate

buf-lint:
	@buf lint

deps:
	@go install github.com/bufbuild/buf/cmd/buf@latest
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest


clean:
	@go mod tidy
	rm -rf $(BIN_DIR)
	rm -rf proto/gen
	rm -rf store.db
	rm -rf third_party/*

.PHONY: all controller docker-controller deps frontend buf-lint clean node all
