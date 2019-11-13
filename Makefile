PROTOS := sample/user.proto
PBGO := $(PROTOS:.proto=.pb.go)

all: $(PBGO)
	go build .
	go build -o sample/sample ./sample

%.pb.go: %.proto
	protoc --go_out=. $<

.PHONY: all