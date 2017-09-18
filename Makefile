build:
	@echo "building tunebot"
	go build -o tunebot

build-static:
	@echo "building static tunebot"
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' .

clean:
	@echo "cleaning"
	rm -f tunebot
	rm -rf tunebot-repo

run:
	@echo "running tunebot"
	go run main.go

.default: clean build