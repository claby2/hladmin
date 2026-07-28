mod exec;
mod pull;
mod push_staged;
mod rebuild;
mod reset;
mod resolve;
mod status;

use crate::cli::Commands;
use crate::config;
use crate::executor;
use crate::ui;
use anyhow::{Result, anyhow, bail};

pub fn dispatch(command: Commands) -> Result<()> {
    match command {
        Commands::Exec {
            interactive,
            hosts,
            command,
        } => exec::run(interactive, &hosts, &command),
        Commands::Status { hosts } => status::run(&hosts),
        Commands::Rebuild { remote, hosts } => rebuild::run(remote, &hosts),
        Commands::Pull { hosts } => pull::run(&hosts),
        Commands::PushStaged { dry_run, hosts } => push_staged::run(dry_run, &hosts),
        Commands::Reset { yes, hosts } => reset::run(yes, &hosts),
        Commands::Resolve { hosts } => resolve::run(&hosts),
    }
}

/// Loads the host configuration and resolves the provided arguments (which may
/// include @group syntax) into a flat list of hostnames.
pub(crate) fn resolve_hosts(args: &[String]) -> Result<Vec<String>> {
    let host_config =
        config::load_config().map_err(|e| anyhow!("failed to load host configuration: {e}"))?;

    let hostnames = host_config
        .resolve_hosts(args)
        .map_err(|e| anyhow!("failed to resolve hosts: {e}"))?;

    if hostnames.is_empty() {
        bail!("at least one hostname must be specified");
    }

    Ok(hostnames)
}

/// Runs command on hosts with streaming output and returns the first host
/// error, if any.
pub(crate) fn stream_command(hostnames: &[String], command: &str) -> Result<()> {
    let results = ui::stream::run_streaming(hostnames, command)?;
    executor::results_error(&results)
}
