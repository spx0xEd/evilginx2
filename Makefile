TARGET=evilginx
PACKAGES=core database log parser

.PHONY: all build clean
all: build

build:
	@go build -trimpath -ldflags="-s -w"  -o ./$(TARGET) -mod=vendor .

clean:
	@go clean
	@rm -f ./$(TARGET)
	@rm -rf ./run.sh ./evilginx.service
