CMDS := $(notdir $(wildcard cmd/*))
PKG := ./...

.PHONY: build test vet fmt clean tidy $(CMDS)

build:
	@mkdir -p bin
	@for c in $(CMDS); do \
		echo "building $$c"; \
		CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/$$c ./cmd/$$c || exit 1; \
	done

$(CMDS):
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/$@ ./cmd/$@

test:
	go test -race -count=1 $(PKG)

vet:
	go vet $(PKG)

fmt:
	gofmt -w .

tidy:
	go mod tidy

clean:
	rm -rf bin
