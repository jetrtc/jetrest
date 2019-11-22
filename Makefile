PROTOS := $(wildcard */*.proto)
PBGO := $(PROTOS:.proto=.pb.go)

all: $(PBGO)
	go build .
	go build -o sample/sample ./sample

run:
	go run ./sample

test:
	go test .
	go test ./sample

%.pb.go: %.proto
	protoc --go_out=. $<

.PHONY: all run test