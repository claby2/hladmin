use crate::commands::resolve_hosts;
use crate::executor::describe_exit;
use crate::ui::styles;
use anyhow::{Result, bail};
use std::io::Write;
use std::process::{Command, Stdio};

/// Runs a command discarding all output, returning a Go-style error description
/// on failure.
fn run_quiet(cmd: &mut Command) -> std::result::Result<(), String> {
    match cmd
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status()
    {
        Ok(status) if status.success() => Ok(()),
        Ok(status) => Err(describe_exit(status)),
        Err(e) => Err(e.to_string()),
    }
}

/// Runs a command capturing combined stdout+stderr, returning the output and a
/// Go-style error description on failure.
fn run_combined(cmd: &mut Command) -> (String, Option<String>) {
    match cmd.stdin(Stdio::null()).output() {
        Ok(output) => {
            let mut combined = String::from_utf8_lossy(&output.stdout).into_owned();
            combined.push_str(&String::from_utf8_lossy(&output.stderr));
            let err = (!output.status.success()).then(|| describe_exit(output.status));
            (combined, err)
        }
        Err(e) => (String::new(), Some(e.to_string())),
    }
}

pub fn run(dry_run: bool, hosts: &[String]) -> Result<()> {
    let hostnames = resolve_hosts(hosts)?;

    let Some(home) = std::env::var_os("HOME").filter(|h| !h.is_empty()) else {
        bail!("HOME environment variable not set");
    };
    let nix_config_path = std::path::Path::new(&home).join("nix-config");

    // Check for staged changes with --binary to handle binary files properly.
    let diff_output = Command::new("git")
        .args(["diff", "--cached", "--binary"])
        .current_dir(&nix_config_path)
        .stdin(Stdio::null())
        .output();
    let diff_output = match diff_output {
        Ok(output) if output.status.success() => output.stdout,
        Ok(output) => bail!(
            "failed to check staged changes in {}: {}",
            nix_config_path.display(),
            describe_exit(output.status)
        ),
        Err(e) => bail!(
            "failed to check staged changes in {}: {}",
            nix_config_path.display(),
            e
        ),
    };

    if diff_output.is_empty() {
        println!("{}", styles::info().apply_to("No staged changes found"));
        return Ok(());
    }

    if dry_run {
        println!("{}", styles::header().apply_to("Staged changes:"));
        println!("{}", String::from_utf8_lossy(&diff_output));
        println!();
    }

    // Local patch file; removed automatically when dropped.
    let mut patch_file = tempfile::Builder::new()
        .prefix("hladmin-patch-")
        .suffix(".patch")
        .tempfile()
        .map_err(|e| anyhow::anyhow!("failed to create temp file: {e}"))?;
    patch_file
        .write_all(&diff_output)
        .map_err(|e| anyhow::anyhow!("failed to write patch file: {e}"))?;
    patch_file
        .flush()
        .map_err(|e| anyhow::anyhow!("failed to write patch file: {e}"))?;

    for hostname in &hostnames {
        println!(
            "{} {}",
            styles::info().apply_to("Processing host:"),
            styles::hostname().apply_to(hostname)
        );

        // Check if the remote repo is clean.
        let (clean_output, clean_err) = run_combined(
            Command::new("ssh")
                .arg(hostname)
                .arg("cd $HOME/nix-config && git status --porcelain"),
        );
        if let Some(err) = clean_err {
            println!(
                "{}",
                styles::error()
                    .apply_to(format!("  Error checking git status on {hostname}: {err}"))
            );
            continue;
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
            continue;
        }

        if dry_run {
            println!(
                "{}",
                styles::success().apply_to("  Repository is clean, would apply patch")
            );
            continue;
        }

        // Unique remote path (hostname + PID) prevents conflicts when multiple
        // hladmin instances target the same host.
        let remote_patch_file =
            format!("/tmp/hladmin-patch-{hostname}-{}.patch", std::process::id());

        if let Err(err) = run_quiet(
            Command::new("scp")
                .arg(patch_file.path())
                .arg(format!("{hostname}:{remote_patch_file}")),
        ) {
            println!(
                "{}",
                styles::error().apply_to(format!("  Error copying patch: {err}"))
            );
            continue;
        }

        let (apply_output, apply_err) = run_combined(Command::new("ssh").arg(hostname).arg(
            format!("cd $HOME/nix-config && git apply {remote_patch_file}"),
        ));

        // Always clean up the remote patch file, regardless of git apply result.
        let _ = run_quiet(
            Command::new("ssh")
                .arg(hostname)
                .arg(format!("rm -f {remote_patch_file}")),
        );

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
            continue;
        }

        println!(
            "{}",
            styles::success().apply_to("  Patch applied successfully")
        );
    }

    Ok(())
}
