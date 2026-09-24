BIN_DIR := ./bin
BINARY := $(BIN_DIR)/devreport
SRC := ./cmd/devreport
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
LDFLAGS := -ldflags="-s -w"

# ── env defaults (override via env or inline)
GITHUB_TOKEN     ?= $(shell echo $$GITHUB_TOKEN)
GITHUB_ORGS      ?= $(shell echo $$GITHUB_ORGS)
GITHUB_USERNAME  ?= $(shell echo $$GITHUB_USERNAME)
GITHUB_REPOS     ?= $(shell echo $$GITHUB_REPOS)

CLICKUP_TOKEN    ?= $(shell echo $$CLICKUP_API_KEY)
CLICKUP_ASSIGNEES?= $(shell echo $$CLICKUP_ASSIGNEE_IDS)
CLICKUP_LISTIDS  ?= $(shell echo $$CLICKUP_LISTIDS)
CLICKUP_FOLDERID ?= $(shell echo $$CLICKUP_FOLDERID)

REPORT_USER      ?= $(shell echo $$REPORT_USER)
REPORT_AUTHOR    ?= $(shell echo $$REPORT_AUTHOR)
REPORT_START     ?= $(shell date -d '7 days ago' +%Y-%m-%d 2>/dev/null || date -v-7d +%Y-%m-%d)
REPORT_END       ?= $(shell date +%Y-%m-%d)
REPORT_PERIOD    ?= $(shell date +%B)
REPORT_YEAR      ?= $(shell date +%Y)
REPORT_OUTPUT    ?= reports

LLM_PROVIDER     ?= $(shell echo $$LLM_PROVIDER)
CLAUDE_API_KEY   ?= $(shell echo $$ANTHROPIC_API_KEY)
CLAUDE_MODEL     ?= $(shell echo $$CLAUDE_MODEL)
OLLAMA_MODEL     ?= $(shell echo $$OLLAMA_MODEL)

CLUSTER_INPUT    ?= $(shell echo $$CLUSTER_INPUT)
CLUSTER_MIN_GROUP?= $(shell echo $$CLUSTER_MIN_GROUP)
CLUSTER_BATCH    ?= $(shell echo $$CLUSTER_BATCH)

.PHONY: help all build build/linux build/mac build/windows run clean \
        run/github run/clickup run/all summary/clickup cluster

## help: Show this help message
help:
	@echo "Available commands:"
	@grep -E '^##' $(MAKEFILE_LIST) | sed -e 's/^## //' | column -t -s ':' | sort

## all: Build the binary for the current platform
all: build

## build: Build the binary for the current platform
build:
	@mkdir -p $(BIN_DIR)
	@echo "Building devreport for $(GOOS)/$(GOARCH)..."
	@go build $(LDFLAGS) -o $(BINARY) $(SRC)
	@echo "Binary created at $(BINARY)"

## build/linux: Build the binary for Linux amd64
build/linux:
	@mkdir -p $(BIN_DIR)/linux_amd64
	@echo "Building devreport for linux/amd64..."
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/linux_amd64/devreport $(SRC)
	@echo "Binary created at $(BIN_DIR)/linux_amd64/devreport"

## build/mac: Build the binary for macOS amd64
build/mac:
	@mkdir -p $(BIN_DIR)/darwin_amd64
	@echo "Building devreport for darwin/amd64..."
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/darwin_amd64/devreport $(SRC)
	@echo "Binary created at $(BIN_DIR)/darwin_amd64/devreport"

## build/windows: Build the binary for Windows amd64
build/windows:
	@mkdir -p $(BIN_DIR)/windows_amd64
	@echo "Building devreport for windows/amd64..."
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/windows_amd64/devreport.exe $(SRC)
	@echo "Binary created at $(BIN_DIR)/windows_amd64/devreport.exe"

## run/github: Generate report from GitHub only
run/github: build
	@echo "Running GitHub report for $(REPORT_USER) ($(REPORT_START) -> $(REPORT_END))..."
	@$(BINARY) \
		--user "$(REPORT_USER)" \
		--author "$(REPORT_AUTHOR)" \
		--start "$(REPORT_START)" \
		--end "$(REPORT_END)" \
		--period "$(REPORT_PERIOD)" \
		--year "$(REPORT_YEAR)" \
		--output "$(REPORT_OUTPUT)" \
		--github-token "$(GITHUB_TOKEN)" \
		--github-orgs "$(GITHUB_ORGS)" \
		--github-username "$(GITHUB_USERNAME)" \
		$(if $(GITHUB_REPOS),--github-repos "$(GITHUB_REPOS)",) \
		$(if $(LLM_PROVIDER),--llm-provider "$(LLM_PROVIDER)",) \
		$(if $(CLAUDE_API_KEY),--claude-api-key "$(CLAUDE_API_KEY)",) \
		$(if $(CLAUDE_MODEL),--claude-model "$(CLAUDE_MODEL)",)

## run/clickup: Generate report from ClickUp only
run/clickup: build
	@echo "Running ClickUp report for $(REPORT_USER) ($(REPORT_START) -> $(REPORT_END))..."
	@$(BINARY) \
		--user "$(REPORT_USER)" \
		--author "$(REPORT_AUTHOR)" \
		--start "$(REPORT_START)" \
		--end "$(REPORT_END)" \
		--period "$(REPORT_PERIOD)" \
		--year "$(REPORT_YEAR)" \
		--output "$(REPORT_OUTPUT)" \
		--clickup-token "$(CLICKUP_TOKEN)" \
		--clickup-assignees "$(CLICKUP_ASSIGNEES)" \
		$(if $(CLICKUP_FOLDERID),--clickup-folderid "$(CLICKUP_FOLDERID)",) \
		$(if $(CLICKUP_LISTIDS),--clickup-listid "$(CLICKUP_LISTIDS)",) \
		$(if $(LLM_PROVIDER),--llm-provider "$(LLM_PROVIDER)",) \
		$(if $(CLAUDE_API_KEY),--claude-api-key "$(CLAUDE_API_KEY)",) \
		$(if $(CLAUDE_MODEL),--claude-model "$(CLAUDE_MODEL)",)

## run/all: Generate report from both GitHub and ClickUp
run/all: build
	@echo "Running full report for $(REPORT_USER) ($(REPORT_START) -> $(REPORT_END))..."
	@$(BINARY) \
		--user "$(REPORT_USER)" \
		--author "$(REPORT_AUTHOR)" \
		--start "$(REPORT_START)" \
		--end "$(REPORT_END)" \
		--period "$(REPORT_PERIOD)" \
		--year "$(REPORT_YEAR)" \
		--output "$(REPORT_OUTPUT)" \
		--github-token "$(GITHUB_TOKEN)" \
		--github-orgs "$(GITHUB_ORGS)" \
		--github-username "$(GITHUB_USERNAME)" \
		$(if $(GITHUB_REPOS),--github-repos "$(GITHUB_REPOS)",) \
		--clickup-token "$(CLICKUP_TOKEN)" \
		--clickup-assignees "$(CLICKUP_ASSIGNEES)" \
		$(if $(CLICKUP_FOLDERID),--clickup-folderid "$(CLICKUP_FOLDERID)",) \
		$(if $(CLICKUP_LISTIDS),--clickup-listid "$(CLICKUP_LISTIDS)",) \
		$(if $(LLM_PROVIDER),--llm-provider "$(LLM_PROVIDER)",) \
		$(if $(CLAUDE_API_KEY),--claude-api-key "$(CLAUDE_API_KEY)",) \
		$(if $(CLAUDE_MODEL),--claude-model "$(CLAUDE_MODEL)",)

## summary/clickup: Generate team summary (Excel) from ClickUp
summary/clickup: build
	@echo "Running ClickUp team summary ($(REPORT_PERIOD))..."
	@$(BINARY) summary \
		--clickup-token "$(CLICKUP_TOKEN)" \
		$(if $(CLICKUP_FOLDERID),--clickup-folderid "$(CLICKUP_FOLDERID)",) \
		$(if $(CLICKUP_LISTIDS),--clickup-listid "$(CLICKUP_LISTIDS)",) \
		$(if $(CLICKUP_ASSIGNEES),--clickup-assignees "$(CLICKUP_ASSIGNEES)",) \
		$(if $(LLM_PROVIDER),--llm-provider "$(LLM_PROVIDER)",) \
		$(if $(CLAUDE_API_KEY),--claude-api-key "$(CLAUDE_API_KEY)",) \
		$(if $(CLAUDE_MODEL),--claude-model "$(CLAUDE_MODEL)",) \
		--csv "$(REPORT_OUTPUT)"

## cluster: Merge related/small tasks in an existing report (CLUSTER_INPUT=path.json|csv) into fewer rows
cluster: build
	@if [ -z "$(CLUSTER_INPUT)" ]; then echo "CLUSTER_INPUT is required, e.g. make cluster CLUSTER_INPUT=reports/report_x.json"; exit 1; fi
	@echo "Clustering $(CLUSTER_INPUT)..."
	@$(BINARY) cluster \
		--input "$(CLUSTER_INPUT)" \
		--output "$(REPORT_OUTPUT)" \
		--author "$(REPORT_AUTHOR)" \
		$(if $(LLM_PROVIDER),--llm-provider "$(LLM_PROVIDER)",) \
		$(if $(CLAUDE_API_KEY),--claude-api-key "$(CLAUDE_API_KEY)",) \
		$(if $(CLAUDE_MODEL),--claude-model "$(CLAUDE_MODEL)",) \
		$(if $(OLLAMA_MODEL),--ollama-model "$(OLLAMA_MODEL)",) \
		$(if $(CLUSTER_MIN_GROUP),--min-group "$(CLUSTER_MIN_GROUP)",) \
		$(if $(CLUSTER_BATCH),--batch-size "$(CLUSTER_BATCH)",)

## run: Run the built binary (no args)
run:
	@echo "Running devreport..."
	@$(BINARY)

## clean: Remove built binaries
clean:
	@echo "Cleaning binaries..."
	@rm -rf $(BIN_DIR)
	@echo "Done."
