# devreport

[![Go](https://img.shields.io/badge/Go-1.21%2B-blue.svg)](https://golang.org/) [![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**DevReport** is a command-line tool that generates automated individual activity reports from ClickUp tasks. It pulls tasks, computes stats (total, completed, by status/source/type), and renders beautiful HTML/PDF reports with summaries and detailed tables.

## Features

- **One-command reports**: Fetch from ClickUp → Generate HTML/PDF + JSON export  
- **Smart stats**: Auto-calculates totals, completion rates, breakdowns by status/source/type  
- **Cross-platform**: Pre-built binaries for macOS, Linux, Windows (AMD64/ARM64)  
- **Optional AI Rephrasing**: Integrates with [Ollama](https://ollama.com/) for task rewording  

## Prerequisites

- **ClickUp Account**: Access to tasks via API  
- **Terminal**: sh/Zsh/PowerShell  
- **Optional**: Browser to view HTML reports  
- **Optional**: [Ollama](https://ollama.com/) running locally (port 11434) for AI rephrasing  

## Installing Ollama and Gemma3 on linux

1. Install Ollama:

   ```sh
   curl -fsSL https://ollama.com/install.sh | sh
   ```

2. Start Ollama:

   ```sh
   ollama serve
   ```

3. Pull the Gemma3 model:

   ```sh
   ollama pull gemma3
   ```

4. Verify installation:

   ```sh
   curl http://localhost:11434/api/tags
   ```

---

## Installation

### Option 1: Download Pre-built Binary (Recommended)

1. Go to the [Releases page](https://github.com/Afrawles/devreport/releases)
2. Download the archive for your platform:
   - macOS Intel: `devreport_darwin_amd64.tar.gz`
   - macOS Apple Silicon: `devreport_darwin_arm64.tar.gz`
   - Linux AMD64: `devreport_linux_amd64.tar.gz`
   - Linux ARM64: `devreport_linux_arm64.tar.gz`
   - Windows: `devreport_windows_amd64.zip`

3. Extract and install (macOS/Linux):

   ```sh
   tar -xzf devreport_darwin_amd64.tar.gz
   chmod +x devreport
   sudo mv devreport /usr/local/bin/
   ```

4. Verify installation:

   ```sh
   devreport --help
   ```

### Option 2: Build from Source

```sh
git clone https://github.com/Afrawles/devreport.git
cd devreport
go build -o devreport ./cmd/devreport
./devreport --help
```

---

## Getting Your ClickUp API Token

1. Log in to [ClickUp](https://app.clickup.com)
2. Click your profile picture → **Settings**
3. Go to **Apps** → **API Token**
4. Click **Generate** and copy your token
5. Use it with the `--clickup-token` flag

---

## Finding Your ClickUp List IDs

1. In the ClickUp sidebar, hover over a List  
2. Click the **ellipsis (...)** → **Copy link**  
3. Example:

   ```sh
   https://app.clickup.com/12345678/v/li/987654321
   ```

4. The number after `/li/` is your **List ID** (`987654321`)  
5. Use commas to separate multiple lists:

   ```sh
   987654321,123456789
   ```

---

## Finding Your ClickUp Assignee IDs

```sh
curl -H "Authorization: YOUR_API_TOKEN" \
  "https://api.clickup.com/api/v2/team"
```

---

## Usage

When working with multiple ClickUp lists, DevReport maps text by list order.

- Use commas (`,`) to separate **different lists**
- Use pipes (`|`) within each list group to separate **sentences belonging to that list**

### Example Layout

```sh
--clickup-listid "11111111,33333333" \
--challenges "Delayed client feedback|Unclear UI specifications|Integration issues with payment service, Server maintenance downtime|Third-party API instability|Deployment delays" \
--support-required "Product team review|QA support for test coverage|DevOps for CI/CD automation, Management alignment|Database admin support|Load testing assistance" \
--support-from "Product Management|QA Department|DevOps Team, IT Infrastructure|Backend Team|Project Management Office" \
--follow-up "Conduct sprint retrospective|Optimize frontend performance|Write integration tests, Refactor legacy modules|Enhance documentation|Evaluate monitoring tools"
```

Explanation:

- Everything before the first comma (`,`) belongs to **List 11111111**  
- Everything after the comma belongs to **List 33333333**  
- Within each group, `|` separates sentences for that list  

---

### Basic Command

```sh
./devreport \
  --user "Uzumaki.Gon" \
  --start "2025-10-01" \
  --end "2025-10-31" \
  --author "Killua Uzumaki" \
  --period "Month of October" \
  --year 2025 \
  --category "Improvements, New Features, and Bug Fixes" \
  --clickup-token "your_clickup_token_here" \
  --clickup-assignees 1234536,1728383 \
  --clickup-listid "11111111,33333333" \
  --challenges "Delayed client feedback|Unclear UI specifications|Integration issues with payment service, Server maintenance downtime|Third-party API instability|Deployment delays" \
  --support-required "Product team review|QA support for test coverage|DevOps for CI/CD automation, Management alignment|Database admin support|Load testing assistance" \
  --support-from "Product Management|QA Department|DevOps Team, IT Infrastructure|Backend Team|Project Management Office" \
  --follow-up "Conduct sprint retrospective|Optimize frontend performance|Write integration tests, Refactor legacy modules|Enhance documentation|Evaluate monitoring tools"
```

After execution, open the generated report:

```sh
reports/report_Uzumaki.Gon_20251030.html
```

This file (report export) contains the task summary, categorized sections, and AI-rephrased content (if Ollama is available).
