VERSION=0.0.1
LDFLAGS=-ldflags "-w -s -X main.version=${VERSION} "

all: sacloud-cpu-usage

.PHONY: sacloud-cpu-usage

sacloud-cpu-usage: main.go
	go build $(LDFLAGS) -o sacloud-cpu-usage main.go

linux: main.go
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o sacloud-cpu-usage main.go

check:
	go test ./...

fmt:
	go fmt ./...

tag:
	git tag v${VERSION}
	git push origin v${VERSION}
	git push origin main
