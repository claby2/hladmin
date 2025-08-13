# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`hladmin` is a homelab administration tool built in Go using the Cobra CLI framework. It manages NixOS servers and macOS machines running nix-darwin by executing commands remotely via SSH or locally when the `--local` flag is used.

## Development Commands

### Building

```bash
go build -o hladmin
```

### Running

```bash
./hladmin <command> [flags] [hosts...]
```

### Dependencies

```bash
go mod tidy
```

## Architecture

### CLI Structure

- **Entry Point**: `main.go` - Simple entry point that calls `cmd.Execute()`
- **Root Command**: `cmd/root.go` - Defines the root `hladmin` command and registers all subcommands
- **Subcommands**: Each command is in its own file under `cmd/`:
  - `push_staged.go` - Pushes staged git changes from local to remote hosts
  - `status.go` - Shows system status in columnar format using tabwriter
  - `pull.go` - Executes git pull on nix-config repositories
  - `rebuild.go` - Runs rebuild.sh script with terminal support (ssh -t)
  - `exec.go` - Executes arbitrary commands on hosts

### Command Pattern

All commands follow a consistent pattern:

1. **Flag Variables**: Declared at package level (e.g., `statusLocal bool`)
2. **Command Definition**: Cobra command struct with Use, Short, Long, RunE
3. **Flag Registration**: `init()` function registers flags
4. **Execution Logic**: `run*()` function handles both local and remote execution
5. **Validation**: Commands require either hostnames OR `--local` flag

### Local vs Remote Execution

Commands support both local and remote execution:

- **Remote**: Uses SSH to execute commands on specified hostnames
- **Local**: When `--local` flag is set, executes commands directly without SSH
- **Mixed**: Can combine `--local` with remote hostnames in single command

### Key Implementation Details

#### SSH Execution

- Standard commands use `ssh hostname "command"`
- Interactive commands (rebuild) use `ssh -t hostname "command"` with stdin/stdout/stderr connected
- Remote paths always use `$HOME/nix-config` (shell variable expansion)

#### Local Execution

- Uses `bash -c "command"` or direct Go `exec.Command()`
- Local paths use `filepath.Join(os.Getenv("HOME"), "nix-config")`
- Git operations set working directory with `cmd.Dir`

#### Special Command Handling

- **exec**: Requires `--` separator between hosts and command args, manually parses `--local` flag due to `DisableFlagParsing: true`
- **push-staged**: Creates temporary patch files, only applies to clean git repos
- **status**: Uses `text/tabwriter` for columnar output, collects system metrics (HOSTCLASS, version, uptime, disk, memory, git status)

### Error Handling

Commands continue processing remaining hosts when individual hosts fail, printing errors but not stopping execution.

### Path Management

- **Local paths**: Always use `os.Getenv("HOME")` + `filepath.Join()` for cross-platform compatibility
- **Remote paths**: Use shell variable `$HOME` for SSH command execution
- **Temporary files**: `push-staged` creates temp directories that are automatically cleaned up

## Homelab Context

The tool manages a homelab consisting of:

- Multiple servers and desktop machines running NixOS
- A Mac running nix-darwin
- All systems have `$HOME/nix-config` directory with a `rebuild.sh` script
- Systems are identified by hostnames and can be accessed via SSH
- Each system has a `$HOSTCLASS` environment variable indicating its role

