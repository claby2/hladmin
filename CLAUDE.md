# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`hladmin` is a homelab administration tool built in Go using the Cobra CLI framework. It manages NixOS servers and macOS machines running nix-darwin by executing commands remotely via SSH or locally using `localhost` as the hostname.

## Development Commands

### Building

```bash
go build -o hladmin
```

### Nix Build

```bash
nix build
```

### Running

```bash
./hladmin <command> [flags] [hosts...]
```

### Dependencies

```bash
go mod tidy
```

### Development Shell

```bash
nix develop
```

## Architecture

### Project Structure

```
hladmin/
├── main.go                    # Simple entry point calling cmd.Execute()
├── cmd/                       # Cobra CLI commands
│   ├── root.go               # Root command definition and subcommand registration
│   ├── exec.go               # Execute arbitrary commands on hosts
│   ├── status.go             # System status in columnar format
│   ├── rebuild.go            # Run rebuild.sh script interactively
│   ├── pull.go               # Execute git pull operations
│   └── push_staged.go        # Push local staged changes to remote hosts
├── internal/executor/         # Command execution engine
│   └── engine.go             # Parallel and interactive execution logic
├── flake.nix                 # Nix build configuration
├── go.mod                    # Go module dependencies
└── README.md                 # User documentation
```

### Execution Engine (`internal/executor`)

The `executor` package provides the core execution functionality:

#### Types
- **`Result`**: Represents command execution result with hostname, command, stdout/stderr, and error
- **`ExecuteOnHostsParallel()`**: Executes commands on multiple hosts concurrently using goroutines
- **`ExecuteOnHostsInteractive()`**: Executes commands sequentially with stdin/stdout/stderr connected
- **`DisplayResults()`**: Formats and displays execution results

#### Execution Logic
- **Local execution**: Uses `bash -c "command"` when hostname is `localhost`
- **Remote execution**: Uses `ssh hostname "command"` for remote hosts
- **Interactive mode**: Uses `ssh -t hostname "command"` with terminal support
- **Error handling**: Individual host failures don't stop overall execution

### Command Pattern

All commands follow a consistent pattern:

1. **Command Definition**: Cobra command struct with Use, Short, Long, RunE
2. **Validation**: Commands require at least one hostname
3. **Execution**: Delegates to executor package for actual command execution
4. **Error Handling**: Continues processing remaining hosts on individual failures

### Key Commands

#### exec (`cmd/exec.go`)
- **Purpose**: Execute arbitrary commands on specified hosts
- **Features**: 
  - Supports `--interactive` flag for terminal interaction
  - Uses `--` separator to distinguish hosts from command arguments
  - Manual flag parsing due to `DisableFlagParsing: true`
  - Default parallel execution, sequential when interactive

#### status (`cmd/status.go`)
- **Purpose**: Display system information in tabular format using `text/tabwriter`
- **Architecture**: 
  - Compound command execution for efficiency (single SSH call per host)
  - Cross-platform memory detection (Linux `free` vs macOS `vm_stat`)
  - Structured data parsing with `|||` delimiter
  - Parallel collection via executor engine

#### rebuild (`cmd/rebuild.go`)
- **Purpose**: Execute `rebuild.sh` script in `$HOME/nix-config`
- **Features**: Interactive execution for real-time feedback during system rebuilds

#### pull (`cmd/pull.go`)
- **Purpose**: Execute `git pull` in `$HOME/nix-config`
- **Features**: Parallel execution for efficiency

#### push-staged (`cmd/push_staged.go`)
- **Purpose**: Push local staged git changes to clean remote repositories
- **Features**:
  - Creates temporary patch files using `os.CreateTemp()`
  - Validates remote repositories are clean before applying changes
  - Secure cleanup of temporary files on both success and failure
  - `--dry-run` flag for testing

### Path Management

- **Local paths**: Use `os.Getenv("HOME")` + `filepath.Join()` for cross-platform compatibility
- **Remote paths**: Use shell variable `$HOME` in SSH commands
- **Working directory**: Commands set `cmd.Dir` for git operations when needed

### Cross-Platform Support

#### Memory Detection (status command)
```go
// Automatically detects and uses appropriate command
if command -v free >/dev/null 2>&1; then 
    free | grep '^Mem:' | awk '{printf "%.0f%%", $3/$2*100}'
else 
    vm_stat | awk '...' # macOS implementation
fi
```

#### Version Detection
- **NixOS**: Uses `nixos-version --configuration-revision`
- **macOS**: Uses `darwin-version --configuration-revision`
- **Fallback**: Returns 'unknown' if neither is available

### Error Handling Philosophy

- **Graceful degradation**: Individual host failures don't stop batch operations
- **User feedback**: Errors are displayed but execution continues for remaining hosts
- **Resource cleanup**: Temporary files are always cleaned up via `defer` statements

### Security Considerations

- **No credential handling**: Relies on SSH key authentication
- **Temporary file security**: Uses `os.CreateTemp()` with unique filenames
- **Path validation**: Uses `filepath.Join()` to prevent path traversal
- **Command injection prevention**: Proper argument handling in executor

## Homelab Context

The tool manages a homelab consisting of:

- Multiple servers and desktop machines running NixOS
- Mac machines running nix-darwin
- All systems have `$HOME/nix-config` directory with a `rebuild.sh` script
- Systems are identified by hostnames and accessed via SSH
- Each system has a `$HOSTCLASS` environment variable indicating its role (e.g., "server", "desktop", "base")

## Development Guidelines

### Adding New Commands

1. Create new file in `cmd/` directory
2. Define Cobra command struct with proper validation
3. Use executor package for command execution
4. Register command in `cmd/root.go` init function
5. Follow existing error handling patterns

### Function Naming

Use descriptive prefixes to avoid naming collisions:
- `runStatus()` in status.go
- `runExec()` in exec.go  
- `runRebuild()` in rebuild.go
- etc.

### Testing Philosophy

- Manual testing across different host types (NixOS/macOS)
- Verify both local and remote execution modes
- Test error conditions and cleanup procedures
- Validate cross-platform compatibility