GO := go
GO_BUILD = CGO_ENABLED=0 $(GO) build
TARGET=seeflow

TEST_TIMEOUT ?= 3s

all: build

test:
	$(GO) test -timeout=$(TEST_TIMEOUT) -race -cover $$($(GO) list ./...)

build:
	$(GO_BUILD) -o $(TARGET)
