tidy:
	go mod tidy

build: tidy
	mkdir -p bin
	go build -o bin/dockerswarm_discover .
clean:
	rm -rf bin || true
