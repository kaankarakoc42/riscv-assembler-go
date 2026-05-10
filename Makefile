## Makefile for the educational rvtoolchain.
##
## Top-level targets:
##   make build      build all three CLIs into ./bin
##   make test       run all unit + integration tests
##   make demos      assemble & link all three demo programs
##   make clean      delete ./bin and ./build

GO     ?= go
BINDIR := bin
BUILD  := build

CLIS   := rvasm rvld rvdump
DEMOS  := blink counter uart

.PHONY: all build test demos clean fmt

all: build test demos

build:
	@mkdir -p $(BINDIR)
	@for c in $(CLIS) ; do \
		echo ">> go build $$c" ; \
		$(GO) build -o $(BINDIR)/$$c ./cmd/$$c ; \
	done

test:
	$(GO) test ./...

demos: build
	@mkdir -p $(BUILD)/blink   $(BUILD)/counter $(BUILD)/uart
	# blink
	./$(BINDIR)/rvasm -o $(BUILD)/blink/start.ro    examples/blink/start.s
	./$(BINDIR)/rvasm -o $(BUILD)/blink/blink.ro    examples/blink/blink.s
	./$(BINDIR)/rvld  -script examples/blink/link.toml \
	                  -o $(BUILD)/blink/blink \
	                  $(BUILD)/blink/start.ro $(BUILD)/blink/blink.ro
	# counter
	./$(BINDIR)/rvasm -o $(BUILD)/counter/start.ro  examples/counter/start.s
	./$(BINDIR)/rvasm -o $(BUILD)/counter/io.ro     examples/counter/io.s
	./$(BINDIR)/rvasm -o $(BUILD)/counter/main.ro   examples/counter/counter.s
	./$(BINDIR)/rvld  -script examples/counter/link.toml \
	                  -o $(BUILD)/counter/counter \
	                  $(BUILD)/counter/start.ro $(BUILD)/counter/main.ro $(BUILD)/counter/io.ro
	# uart
	./$(BINDIR)/rvasm -o $(BUILD)/uart/start.ro     examples/uart/start.s
	./$(BINDIR)/rvasm -o $(BUILD)/uart/io.ro        examples/uart/io.s
	./$(BINDIR)/rvasm -o $(BUILD)/uart/uart.ro      examples/uart/uart.s
	./$(BINDIR)/rvasm -o $(BUILD)/uart/hello.ro     examples/uart/hello.s
	./$(BINDIR)/rvld  -script examples/uart/link.toml \
	                  -o $(BUILD)/uart/hello \
	                  $(BUILD)/uart/start.ro $(BUILD)/uart/hello.ro \
	                  $(BUILD)/uart/uart.ro  $(BUILD)/uart/io.ro

clean:
	rm -rf $(BINDIR) $(BUILD)

fmt:
	$(GO) fmt ./...
