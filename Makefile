BIN_DIR := ./bin
BINARY := $(BIN_DIR)/devreport

SRC := ./cmd/devreport

# Default local machine OS/ARCH
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

LDFLAGS := -ldflags="-s -w"

.PHONY: all build build/linux build/mac build/windows run clean

all: build

build:
	@mkdir -p $(BIN_DIR)
	@echo "Building devreport for $(GOOS)/$(GOARCH)..."
	@go build $(LDFLAGS) -o $(BINARY) $(SRC)
	@echo "Binary created at $(BINARY)"

build/linux:
	@mkdir -p $(BIN_DIR)/linux_amd64
	@echo "Building devreport for linux/amd64..."
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/linux_amd64/devreport $(SRC)
	@echo "Binary created at $(BIN_DIR)/linux_amd64/devreport"

build/mac:
	@mkdir -p $(BIN_DIR)/darwin_amd64
	@echo "Building devreport for darwin/amd64..."
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/darwin_amd64/devreport $(SRC)
	@echo "Binary created at $(BIN_DIR)/darwin_amd64/devreport"

build/windows:
	@mkdir -p $(BIN_DIR)/windows_amd64
	@echo "Building devreport for windows/amd64..."
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/windows_amd64/devreport.exe $(SRC)
	@echo "Binary created at $(BIN_DIR)/windows_amd64/devreport.exe"

run:
	@echo "Running devreport..."
	@$(BINARY)

clean:
	@echo "Cleaning binaries..."
	@rm -rf $(BIN_DIR)
	@echo "Done."
