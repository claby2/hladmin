use crate::commands::resolve_hosts;
use crate::executor::{self, ExecResult};
use crate::ui;
use crate::ui::livetable::{TableSpec, run_live_table};
use crate::ui::render::{format_duration, render_result_block, result_header};
use crate::ui::styles;
use anyhow::{Result, bail};

fn rebuild_command(remote: bool, build_only: bool) -> String {
    let mut command = String::from("cd $HOME/nix-config && ./rebuild.sh");
    if remote {
        command.push_str(" --remote");
    }
    if build_only {
        command.push_str(" --build-only");
    }
    command
}

fn phase_banner(phase: usize, description: &str) -> String {
    styles::header()
        .apply_to(format!("── Phase {phase}/2: {description} ──"))
        .to_string()
}

fn plural(n: usize) -> &'static str {
    if n == 1 { "" } else { "s" }
}

/// Defines the build-phase table: one row per host with a live status cell.
fn build_table_spec() -> TableSpec {
    TableSpec {
        headers: vec!["HOSTNAME", "STATUS", "TIME"],
        completed_row: Box::new(|result| {
            let status = if result.err.is_some() {
                styles::error().apply_to("✗ build failed").to_string()
            } else {
                styles::success().apply_to("✓ built").to_string()
            };
            vec![
                result.hostname.clone(),
                status,
                styles::secondary()
                    .apply_to(format_duration(result.duration))
                    .to_string(),
            ]
        }),
        running_row: Box::new(|host, spinner_frame, elapsed| {
            vec![
                host.to_string(),
                styles::secondary()
                    .apply_to(format!("{spinner_frame} building…"))
                    .to_string(),
                styles::secondary()
                    .apply_to(format_duration(elapsed))
                    .to_string(),
            ]
        }),
    }
}

/// Prints per-host detail after the build table: full sanitized output for
/// failures (or everything with --verbose). Successful builds print nothing —
/// the activation pass replays the (cached) build, so its diff shows there.
fn print_build_detail(result: &ExecResult, verbose: bool) {
    if verbose || result.err.is_some() {
        println!();
        println!("{}", render_result_block(result));
    }
}

pub fn run(remote: bool, interactive: bool, verbose: bool, hosts: &[String]) -> Result<()> {
    let hostnames = resolve_hosts(hosts)?;
    let switch_command = rebuild_command(remote, false);

    if interactive {
        return ui::interactive::run_interactive(&hostnames, &switch_command);
    }

    // Build phase: nh only needs sudo at activation, so builds run in
    // parallel without any credentials. A live table keeps per-host state
    // visible without streaming nh's noisy build output into scrollback.
    let build_command = rebuild_command(remote, true);
    println!(
        "{}",
        phase_banner(
            1,
            &format!(
                "building {} host{}",
                hostnames.len(),
                plural(hostnames.len())
            )
        )
    );
    let results = run_live_table(&hostnames, &build_command, build_table_spec())?;

    // collect_results returns results in input order, so skip messages and
    // error precedence are deterministic.
    let mut first_error = None;
    let mut to_activate = Vec::new();
    for result in &results {
        print_build_detail(result, verbose);
        match &result.err {
            Some(err) => {
                println!(
                    "{}",
                    styles::error().apply_to(format!("{err}, skipping activation"))
                );
                first_error.get_or_insert(err.clone());
            }
            None => to_activate.push(result.hostname.clone()),
        }
    }

    // Activation phase: sequential with stdio connected so each host's sudo
    // prompts inline; the build is already cached, so each pass is short. A
    // failed activation doesn't abort the remaining hosts.
    if !to_activate.is_empty() {
        println!();
        println!(
            "{}",
            phase_banner(
                2,
                &format!(
                    "activating {} host{} (sudo may prompt)",
                    to_activate.len(),
                    plural(to_activate.len())
                )
            )
        );
        for (i, hostname) in to_activate.iter().enumerate() {
            println!();
            println!(
                "{} {}",
                styles::secondary().apply_to(format!("[{}/{}]", i + 1, to_activate.len())),
                result_header(hostname, &switch_command)
            );
            match executor::run_interactive(hostname, &switch_command) {
                Ok(()) => println!("{}", styles::success().apply_to("✓ done")),
                Err(err) => {
                    println!("{}", styles::error().apply_to(&err));
                    first_error.get_or_insert(err.to_string());
                }
            }
        }
    }

    // A failed build must fail the command even when every activation
    // succeeded, and vice versa.
    if let Some(err) = first_error {
        bail!("{err}");
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn rebuild_command_default() {
        assert_eq!(
            rebuild_command(false, false),
            "cd $HOME/nix-config && ./rebuild.sh"
        );
    }

    #[test]
    fn rebuild_command_remote() {
        assert_eq!(
            rebuild_command(true, false),
            "cd $HOME/nix-config && ./rebuild.sh --remote"
        );
    }

    #[test]
    fn rebuild_command_build_only() {
        assert_eq!(
            rebuild_command(false, true),
            "cd $HOME/nix-config && ./rebuild.sh --build-only"
        );
    }

    #[test]
    fn rebuild_command_remote_build_only() {
        assert_eq!(
            rebuild_command(true, true),
            "cd $HOME/nix-config && ./rebuild.sh --remote --build-only"
        );
    }
}
