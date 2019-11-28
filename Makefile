PROTOS := $(wildcard */*.proto)
PBGO := $(PROTOS:.proto=.pb.go)

all: $(PBGO)
	go mod tidy
	go build .
	go build -o sample/sample ./sample

run:
	go run ./sample

test:
	go test -count=1 .
	go test -count=1 ./sample

%.pb.go: %.proto
	protoc --go_out=. $<

.PHONY: all run test