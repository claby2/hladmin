# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`hladmin` is a homelab administration tool built in Rust using the clap CLI framework. It manages NixOS servers and macOS machines running nix-darwin by executing commands remotely via SSH or locally using `localhost` as the hostname.

## Development Commands

### Building

```bash
cargo build
```

### Nix Build

```bash
nix build
```

### Running

```bash
cargo run -- <command> [flags] [hosts...]
```

### Testing

```bash
cargo test
```

Unit tests cover config parsing/resolution, duration formatting, and status output parsing. End-to-end behavior relies on manual testing across different host types.

### Formatting and Linting

```bash
cargo fmt
cargo clippy
```

### Development Shell

```bash
nix develop
```

## Architecture

### Project Structure

```
hladmin/
├── Cargo.toml                 # Crate manifest and dependencies
├── Cargo.lock                 # Locked dependencies (committed; required by buildRustPackage)
├── src/
│   ├── main.rs                # Entry point: parse CLI, dispatch, print "Error: ..." and exit 1
│   ├── cli.rs                 # clap derive command tree (all commands, flags, help text)
│   ├── commands/              # One module per subcommand
│   │   ├── mod.rs             # dispatch() plus shared helpers resolve_hosts()/stream_command()
│   │   ├── exec.rs            # Execute arbitrary commands on hosts (-- separator handling)
│   │   ├── status.rs          # System status in a live columnar table
│   │   ├── rebuild.rs         # Run rebuild.sh script interactively
│   │   ├── pull.rs            # Execute git pull operations
│   │   ├── push_staged.rs     # Push local staged changes to remote hosts
│   │   └── resolve.rs         # Show host configuration and resolve groups
│   ├── config.rs              # Host groups and config file parsing
│   ├── executor.rs            # Command execution engine (local sh / ssh, parallel threads)
│   └── ui/
│       ├── styles.rs          # console-based color palette, NO_COLOR handling, TTY detection
│       ├── render.rs          # format_duration, result blocks with per-host prefixes
│       ├── stream.rs          # indicatif MultiProgress: per-host spinners + scrollback blocks
│       ├── livetable.rs       # comfy-table live table redrawn in place (TableSpec)
│       └── interactive.rs     # Sequential execution with inherited stdio
├── flake.nix                  # Nix build configuration (rustPlatform.buildRustPackage)
└── README.md                  # User documentation
```

### Key Dependencies

- **clap** (derive): CLI parsing and help generation
- **indicatif**: spinners, live redraw regions, and scrollback printing (`MultiProgress::println`)
- **console**: ANSI styling with a global on/off switch (`set_colors_enabled`)
- **comfy-table** (custom_styling feature): borderless table layout with ANSI-aware cell widths
- **indexmap**: preserves hosts-file group order for `resolve` output
- **tempfile**: temporary patch files for push-staged
- **anyhow**: error propagation; `main` prints `Error: {err}` to stderr and exits 1

Concurrency is plain `std::thread` + `mpsc` — the tool only spawns blocking `ssh`
subprocesses, so no async runtime is used. The UI thread drives animation by
looping on `recv_timeout(100ms)`: a timeout is a spinner tick, a message is a
host completion.

### Host Configuration System (`src/config.rs`)

#### Configuration File Format

- **Location**: `$XDG_CONFIG_HOME/hladmin/hosts` or `~/.config/hladmin/hosts`
- **Syntax**: Line-based format with directives
- **Group definition**: `group groupname host1 host2 host3`
- **Default group**: `default groupname` (used when no hosts specified)
- **Comments**: Lines starting with `#` are ignored
- **Missing file**: treated as an empty configuration, not an error

#### Host Resolution

- **Individual hosts**: Specified directly by hostname
- **Group references**: `@groupname` expands to all hosts in group
- **Default handling**: Empty args use default group if configured
- **Deduplication**: Automatic removal of duplicate hostnames (first-occurrence order)
- **Validation**: Unknown groups return errors

### Execution Engine (`src/executor.rs`)

- **`ExecResult`**: hostname, command, stdout/stderr, optional error string, duration
- **`run_on_host()`**: captures output; `sh -c "command"` for `localhost`, `ssh hostname "command"` for remote hosts
- **`run_interactive()`**: inherits stdio; `ssh -t hostname "command"` for remote hosts
- **`run_parallel()`**: validates hosts/command, then one thread per host, completions reported over an mpsc channel as `(input index, result)`
- **`results_error()`**: first error in input order propagates to the exit code
- **Error text**: `error executing on <host>: exit status: <n>` (std `ExitStatus` Display)

### Terminal UI (`src/ui/`)

Three modes:

1. **Streaming** (`stream.rs`, used by exec/pull): an indicatif `MultiProgress`
   shows one spinner line per running host (`<spinner> hostname  elapsed`,
   using indicatif's default spinner frames);
   each completed host's block is pushed into scrollback via `MultiProgress::println`.
   Block format: `===» host` / `cmd: <command>` / `host | <line>` prefixed output /
   green `✓ done <duration>` or red error footer.
2. **Live table** (`livetable.rs`, used by status): one hidden indicatif bar is
   used as a multi-line redraw region; every tick the whole table is rebuilt with
   comfy-table and set as the bar's message. The caller supplies a `TableSpec`
   (headers + completed_row/running_row closures) so command code owns column
   semantics while the UI owns layout and animation.
3. **Interactive** (`interactive.rs`, used by rebuild/exec -i): sequential
   execution with inherited stdio; the child (ssh -t) owns the terminal, so no
   animation library is involved. First failure aborts remaining hosts.

**Color/TTY policy** (`styles.rs`): decided once at startup — colors only when
stdout is a TTY and neither `NO_COLOR` nor `HLADMIN_NO_COLOR` is set (non-empty).
When stdout is not a TTY there is no animation: streaming prints blocks as they
finish, the table renders once at the end.

### Key Commands

- **exec**: `hladmin exec [-i] <hosts...> -- <command> [args...]`. clap's
  `last = true` binds everything after `--` to the command; missing `--` and
  empty command produce the same error messages as always. `-i` runs
  interactively (sequential), default is parallel streaming.
- **status**: single compound SSH command per host producing 5 fields split by
  `|||` (HOSTCLASS, nixos/darwin config revision, nix-config flake revision,
  disk %, memory %). Cross-platform memory detection (Linux `free` vs macOS
  `vm_stat`). VERSION cell is green when it matches REPO, else yellow; parse
  failures render "error" in every column.
- **rebuild**: interactive `cd $HOME/nix-config && ./rebuild.sh`; `--remote`
  appends ` --remote`.
- **pull**: parallel streaming `cd $HOME/nix-config && git pull`.
- **push-staged**: local `git diff --cached --binary`, per-host clean check
  (`git status --porcelain`), scp patch to `/tmp/hladmin-patch-<host>-<pid>.patch`,
  `git apply`, unconditional remote cleanup. `--dry-run`/-n previews. `localhost`
  is skipped with a message (the staged changes originate there). Per-host
  errors print inline and never abort other hosts.
- **resolve**: shows config path, groups (file order), default group; with args,
  shows per-@group expansion and the final deduplicated host list.

### Error Handling Philosophy

- **Graceful degradation**: Individual host failures don't stop batch operations
- **User feedback**: Errors are displayed but execution continues for remaining hosts
- **Resource cleanup**: Temporary files are cleaned up automatically (tempfile drop, unconditional remote rm)

## Homelab Context

The tool manages a homelab consisting of:

- Multiple servers and desktop machines running NixOS
- Mac machines running nix-darwin
- All systems have `$HOME/nix-config` directory with a `rebuild.sh` script
- Systems are identified by hostnames and accessed via SSH
- Each system has a `$HOSTCLASS` environment variable indicating its role (e.g., "server", "desktop", "base")

### Host Groups Configuration

Create `~/.config/hladmin/hosts` to define host groups:

```
# Example configuration
group servers altaria onix golem
group desktops machamp laptop
group all altaria onix golem machamp laptop localhost
default all
```

## Development Guidelines

### Adding New Commands

1. Add a variant to `Commands` in `src/cli.rs` with help text
2. Create a new module in `src/commands/` with a `run()` function
3. Use `commands::resolve_hosts()` for host resolution and validation
4. Use the executor/ui modules for command execution
5. Wire the variant into `dispatch()` in `src/commands/mod.rs`
6. Follow existing error handling patterns (anyhow, exact user-facing messages)

### Testing Philosophy

- Unit tests for pure logic (config parsing, output parsing, formatting)
- Manual testing across different host types (NixOS/macOS)
- Verify both local and remote execution modes
- Verify TTY and non-TTY (piped) output paths

## Troubleshooting

### Nix Build Issues

#### Problem: cargoHash Mismatch

**Symptoms:** `nix build` fails with a hash mismatch error mentioning the vendor
derivation, or complains it cannot fetch crates.

**Root Cause:** The `cargoHash` in `flake.nix` is outdated and doesn't match the
current Cargo dependencies. This happens whenever `Cargo.lock` changes
(dependencies added/updated/removed).

**Solution:**

1. Set a dummy hash in flake.nix:
   `cargoHash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";`
2. Run `nix build` — the error message reports the correct hash under "got:"
3. Update flake.nix with the correct hash
4. Run `nix build` again to complete the build

Also note: `Cargo.lock` must be committed (staged at minimum) — nix flakes only
see git-tracked files, and `buildRustPackage` requires the lockfile.
