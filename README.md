# devreport

[![Go](https://img.shields.io/badge/Go-1.21%2B-blue.svg)](https://golang.org/) [![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**DevReport** is a command-line tool that generates automated individual activity reports from ClickUp tasks. It pulls tasks, computes stats (total, completed, by status/source/type), and renders beautiful HTML/PDF reports with summaries and detailed tables.

## Features

- **One-command reports**: Fetch from ClickUp → Generate HTML/PDF + JSON export
- **Smart stats**: Auto-calculates totals, completion rates, breakdowns by status/source/type
- **Cross-platform**: Pre-built binaries for macOS, Linux, Windows (AMD64/ARM64)

## Prerequisites

- **ClickUp Account**: Access to tasks via API
- **Terminal**: Bash/Zsh/PowerShell
- **Optional**: Browser to view HTML reports

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
   ```bash
   # Extract the archive
   tar -xzf devreport_darwin_amd64.tar.gz
   
   # Make it executable
   chmod +x devreport
   
   # Move to PATH
   sudo mv devreport /usr/local/bin/
   ```
   
   For Windows, extract the ZIP file and add the folder to your PATH.

4. Verify installation:
   ```bash
   devreport --help
   ```

### Option 2: Build from Source

```bash
git clone https://github.com/Afrawles/devreport.git
cd devreport
go build -o devreport ./cmd/devreport
./devreport --help
```

## Getting Your ClickUp API Token

1. Log in to your [ClickUp account](https://app.clickup.com)
2. Click your **profile picture** in the bottom left corner
3. Select **Settings**
4. Navigate to **Apps** in the left sidebar
5. Scroll down to **API Token** section
6. Click **Generate** (or **Regenerate** if you already have one)
7. **Copy the token** - you'll need this for the `--clickup-token` flag

## Finding Your ClickUp List IDs

1. In the **Sidebar**, hover over the **List** you want to pull tasks from
2. Click the **ellipsis (...)** menu
3. Select **Copy link**
4. The copied URL will look like:
    
    ```
    https://app.clickup.com/12345678/v/li/987654321
    ```
    
5. The number after `/li/` is your **List ID** (e.g., `987654321`)
6. For multiple lists, use comma-separated IDs: `987654321,123456789`

## Finding Your ClickUp Assignee IDs

   ```bash
   curl -H "Authorization: YOUR_API_TOKEN" \
     "https://api.clickup.com/api/v2/team"
   ```

## Usage

### Basic Command

```bash
./devreport \
  --user "Afrawles" \
  --start 2025-10-01 \
  --end 2025-10-30 \
  --clickup-token "clickup_api_token_here" \
  --clickup-listid "12345678,87654321" \
  --clickup-assignees "123456789,987654321"
  --author "Uzumaki Saitama"
```

Open `reports/report_Afrawles_20251030.html` in your broswer to view report:
