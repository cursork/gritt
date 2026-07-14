.PHONY: build test test-go test-aplsock vet fmt clean

# Build the main gritt binary. (Test scripts build any grittles they
# need into /tmp — we don't produce grittle binaries here.)
build:
	go build -o gritt .

# Verify everything compiles without producing binaries — useful in CI
# and as a quick "did I break a grittle" check.
build-all:
	go build ./...

test: test-go test-aplsock

# Go test suite. TestTUI builds gritt itself and spawns its own Dyalog
# on a random port, so no separate `build` dep here. Skips if Dyalog
# isn't installed.
test-go:
	go test -v ./...

# Shell-driven aplsock protocol test. Tracks its own PIDs (no pkill
# dyalog) and builds the binaries it needs into /tmp.
test-aplsock:
	bash grittles/aplsock/test.sh

vet:
	go vet ./...

fmt:
	gofmt -l -w .

clean:
	rm -f gritt
