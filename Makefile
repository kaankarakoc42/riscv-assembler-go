## Makefile for the educational rvtoolchain.
##
## Cross-platform: works on both POSIX shells (sh/bash on Linux/macOS/WSL)
## and Windows cmd.exe (which is what GNU make invokes by default on Win).
##
## Top-level targets:
##   make build        compile rvasm, rvld, rvdump into ./bin
##   make test         run all unit + integration tests
##   make demos        assemble & link blink, counter, uart demos
##   make clean        remove ./bin and ./build
##   make fmt          gofmt all sources

GO     ?= go
BINDIR := bin
BUILD  := build

# ─── Platform-specific helpers ───────────────────────────────────────────
# GNU make sets the OS variable to Windows_NT on Windows; everywhere else
# it's empty. We use that to switch between cmd-style and sh-style commands.
ifeq ($(OS),Windows_NT)
  EXE   := .exe
  # `if not exist` is the cmd idiom for "create dir only when missing".
  # The leading `-` tells make to ignore non-zero exits anyway.
  MKDIR  = -@if not exist "$(subst /,\,$(1))" mkdir "$(subst /,\,$(1))"
  RMDIR  = -@if exist "$(subst /,\,$(1))" rmdir /s /q "$(subst /,\,$(1))"
else
  EXE   :=
  MKDIR  = @mkdir -p "$(1)"
  RMDIR  = @rm -rf "$(1)"
endif

RVASM  := $(BINDIR)/rvasm$(EXE)
RVLD   := $(BINDIR)/rvld$(EXE)
RVDUMP := $(BINDIR)/rvdump$(EXE)

.PHONY: all build test demos clean fmt \
        blink-demo counter-demo uart-demo

all: build test demos

# ─── build: compile the three CLI binaries ───────────────────────────────
# Each binary is its own target so make rebuilds only what changed and the
# recipes are short enough to be readable in both shells.
build: $(RVASM) $(RVLD) $(RVDUMP)

$(RVASM):
	$(call MKDIR,$(BINDIR))
	$(GO) build -o $@ ./cmd/rvasm

$(RVLD):
	$(call MKDIR,$(BINDIR))
	$(GO) build -o $@ ./cmd/rvld

$(RVDUMP):
	$(call MKDIR,$(BINDIR))
	$(GO) build -o $@ ./cmd/rvdump

# ─── test: run every unit + integration test ────────────────────────────
test:
	$(GO) test ./...

# ─── demos: assemble & link the three example programs ──────────────────
demos: blink-demo counter-demo uart-demo

blink-demo: build
	$(call MKDIR,$(BUILD)/blink)
	$(RVASM) -o $(BUILD)/blink/start.ro examples/blink/start.s
	$(RVASM) -o $(BUILD)/blink/blink.ro examples/blink/blink.s
	$(RVLD)  -script examples/blink/link.toml \
	         -o $(BUILD)/blink/blink \
	         $(BUILD)/blink/start.ro $(BUILD)/blink/blink.ro

counter-demo: build
	$(call MKDIR,$(BUILD)/counter)
	$(RVASM) -o $(BUILD)/counter/start.ro   examples/counter/start.s
	$(RVASM) -o $(BUILD)/counter/io.ro      examples/counter/io.s
	$(RVASM) -o $(BUILD)/counter/counter.ro examples/counter/counter.s
	$(RVLD)  -script examples/counter/link.toml \
	         -o $(BUILD)/counter/counter \
	         $(BUILD)/counter/start.ro $(BUILD)/counter/counter.ro $(BUILD)/counter/io.ro

uart-demo: build
	$(call MKDIR,$(BUILD)/uart)
	$(RVASM) -o $(BUILD)/uart/start.ro examples/uart/start.s
	$(RVASM) -o $(BUILD)/uart/io.ro    examples/uart/io.s
	$(RVASM) -o $(BUILD)/uart/uart.ro  examples/uart/uart.s
	$(RVASM) -o $(BUILD)/uart/hello.ro examples/uart/hello.s
	$(RVLD)  -script examples/uart/link.toml \
	         -o $(BUILD)/uart/hello \
	         $(BUILD)/uart/start.ro $(BUILD)/uart/hello.ro \
	         $(BUILD)/uart/uart.ro  $(BUILD)/uart/io.ro

# ─── clean ───────────────────────────────────────────────────────────────
clean:
	$(call RMDIR,$(BINDIR))
	$(call RMDIR,$(BUILD))

fmt:
	$(GO) fmt ./...
