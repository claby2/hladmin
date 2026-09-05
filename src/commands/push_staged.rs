use crate::commands::resolve_hosts;
use crate::hostid;
use crate::ui::styles;
use anyhow::{Result, bail};
use std::io::Write;
use std::path::Path;
use std::process::{Command, Stdio};
use tempfile::NamedTempFile;

/// Builds an ssh invocation running remote_command on hostname.
fn ssh_command(hostname: &str, remote_command: &str) -> Command {
    let mut cmd = Command::new("ssh");
    cmd.arg(hostname).arg(remote_command);
    cmd
}

/// Runs a command discarding all output, returning an error description on
/// failure.
fn run_quiet(cmd: &mut Command) -> std::result::Result<(), String> {
    match cmd
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status()
    {
        Ok(status) if status.success() => Ok(()),
        Ok(status) => Err(status.to_string()),
        Err(e) => Err(e.to_string()),
    }
}

/// Runs a command capturing combined stdout+stderr, returning the output and an
/// error description on failure.
fn run_combined(cmd: &mut Command) -> (String, Option<String>) {
    match cmd.stdin(Stdio::null()).output() {
        Ok(output) => {
            let mut combined = String::from_utf8_lossy(&output.stdout).into_owned();
            combined.push_str(&String::from_utf8_lossy(&output.stderr));
            let err = (!output.status.success()).then(|| output.status.to_string());
            (combined, err)
        }
        Err(e) => (String::new(), Some(e.to_string())),
    }
}

/// Returns the staged diff (with --binary to handle binary files properly) of
/// the local nix-config repository.
fn read_staged_diff(nix_config_path: &Path) -> Result<Vec<u8>> {
    let diff_output = Command::new("git")
        .args(["diff", "--cached", "--binary"])
        .current_dir(nix_config_path)
        .stdin(Stdio::null())
        .output();
    match diff_output {
        Ok(output) if output.status.success() => Ok(output.stdout),
        Ok(output) => bail!(
            "failed to check staged changes in {}: {}",
            nix_config_path.display(),
            output.status
        ),
        Err(e) => bail!(
            "failed to check staged changes in {}: {}",
            nix_config_path.display(),
            e
        ),
    }
}

/// Writes the diff to a local patch file; removed automatically when dropped.
fn write_patch_file(diff: &[u8]) -> Result<NamedTempFile> {
    let mut patch_file = tempfile::Builder::new()
        .prefix("hladmin-patch-")
        .suffix(".patch")
        .tempfile()
        .map_err(|e| anyhow::anyhow!("failed to create temp file: {e}"))?;
    patch_file
        .write_all(diff)
        .and_then(|_| patch_file.flush())
        .map_err(|e| anyhow::anyhow!("failed to write patch file: {e}"))?;
    Ok(patch_file)
}

/// Applies the patch to a single host: clean check, scp, git apply, cleanup.
/// Prints its own status lines; errors never propagate so other hosts still run.
fn push_to_host(hostname: &str, patch_path: &Path, dry_run: bool) {
    // The staged changes were generated from this machine's nix-config, so
    // pushing them back to this same machine is meaningless (and would require sshd).
    if hostid::is_self(hostname) {
        println!(
            "{}",
            styles::info().apply_to(format!(
                "  Skipping {hostname}: staged changes originate here"
            ))
        );
        return;
    }

    // Check if the remote repo is clean.
    let (clean_output, clean_err) = run_combined(&mut ssh_command(
        hostname,
        "cd $HOME/nix-config && git status --porcelain",
    ));
    if let Some(err) = clean_err {
        println!(
            "{}",
            styles::error().apply_to(format!("  Error checking git status on {hostname}: {err}"))
        );
        return;
    }

    if !clean_output.trim().is_empty() {
        println!(
            "{}",
            styles::warning().apply_to("  Repository has uncommitted changes, skipping")
        );
        if dry_run {
            println!(
                "{}",
                styles::secondary().apply_to("  Would skip due to uncommitted changes")
            );
        }
        return;
    }

    if dry_run {
        println!(
            "{}",
            styles::success().apply_to("  Repository is clean, would apply patch")
        );
        return;
    }

    // Unique remote path (hostname + PID) prevents conflicts when multiple
    // hladmin instances target the same host.
    let remote_patch_file = format!("/tmp/hladmin-patch-{hostname}-{}.patch", std::process::id());

    let (scp_output, scp_err) = run_combined(
        Command::new("scp")
            .arg(patch_path)
            .arg(format!("{hostname}:{remote_patch_file}")),
    );
    if let Some(err) = scp_err {
        println!(
            "{}",
            styles::error().apply_to(format!("  Error copying patch: {err}"))
        );
        if !scp_output.is_empty() {
            println!(
                "{}",
                styles::secondary().apply_to(format!("  {scp_output}"))
            );
        }
        return;
    }

    let (apply_output, apply_err) = run_combined(&mut ssh_command(
        hostname,
        &format!("cd $HOME/nix-config && git apply '{remote_patch_file}'"),
    ));

    // Always clean up the remote patch file, regardless of git apply result.
    let _ = run_quiet(&mut ssh_command(
        hostname,
        &format!("rm -f '{remote_patch_file}'"),
    ));

    if let Some(err) = apply_err {
        println!(
            "{}",
            styles::error().apply_to(format!("  Error applying patch: {err}"))
        );
        if !apply_output.is_empty() {
            println!(
                "{}",
                styles::secondary().apply_to(format!("  {apply_output}"))
            );
        }
        return;
    }

    println!(
        "{}",
        styles::success().apply_to("  Patch applied successfully")
    );
}

pub fn run(dry_run: bool, hosts: &[String]) -> Result<()> {
    let hostnames = resolve_hosts(hosts)?;

    let Some(home) = std::env::var_os("HOME").filter(|h| !h.is_empty()) else {
        bail!("HOME environment variable not set");
    };
    let nix_config_path = Path::new(&home).join("nix-config");

    let diff = read_staged_diff(&nix_config_path)?;
    if diff.is_empty() {
        println!("{}", styles::info().apply_to("No staged changes found"));
        return Ok(());
    }

    if dry_run {
        println!("{}", styles::header().apply_to("Staged changes:"));
        println!("{}", String::from_utf8_lossy(&diff));
        println!();
    }

    let patch_file = write_patch_file(&diff)?;

    for hostname in &hostnames {
        println!(
            "{} {}",
            styles::info().apply_to("Processing host:"),
            styles::hostname().apply_to(hostname)
        );
        push_to_host(hostname, patch_file.path(), dry_run);
    }

    Ok(())
}
