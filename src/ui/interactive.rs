use crate::executor;
use crate::ui::render::result_header;
use crate::ui::styles;
use anyhow::{Result, bail};

/// Executes command on each host sequentially with stdio connected, printing a
/// styled header and completion marker around each run. The child process
/// (ssh -t) owns the terminal, so no animation is used.
pub fn run_interactive(hosts: &[String], command: &str) -> Result<()> {
    // Without this check an empty host list would silently succeed; the
    // per-host command validation lives in executor::run_interactive.
    if hosts.is_empty() {
        bail!("at least one hostname must be specified");
    }

    for (i, hostname) in hosts.iter().enumerate() {
        if i > 0 {
            println!();
        }
        println!("{}", result_header(hostname, command));
        if let Err(err) = executor::run_interactive(hostname, command) {
            bail!("{}", styles::error().apply_to(err));
        }
        println!("{}", styles::success().apply_to("✓ done"));
    }
    Ok(())
}
