use crate::commands::resolve_hosts;
use crate::executor;
use crate::ui;
use crate::ui::render::result_header;
use crate::ui::styles;
use anyhow::{Result, bail};
use std::collections::HashMap;

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

pub fn run(remote: bool, interactive: bool, hosts: &[String]) -> Result<()> {
    let hostnames = resolve_hosts(hosts)?;
    let switch_command = rebuild_command(remote, false);

    if interactive {
        return ui::interactive::run_interactive(&hostnames, &switch_command);
    }

    // Build phase: nh only needs sudo at activation, so builds run in
    // parallel without any credentials.
    let build_command = rebuild_command(remote, true);
    let results = ui::stream::run_streaming(&hostnames, &build_command)?;

    let mut build_errors = HashMap::new();
    for result in results {
        if let Some(err) = result.err {
            build_errors.insert(result.hostname, err);
        }
    }

    // Results arrive in completion order; walk the input order so skip
    // messages and error precedence are deterministic.
    let mut first_error = None;
    let mut to_activate = Vec::new();
    for hostname in hostnames {
        match build_errors.get(&hostname) {
            Some(err) => {
                println!(
                    "{}",
                    styles::error().apply_to(format!("{err}, skipping activation"))
                );
                first_error.get_or_insert(err.clone());
            }
            None => to_activate.push(hostname),
        }
    }

    // Activation phase: sequential with stdio connected so each host's sudo
    // prompts inline; the build is already cached, so each pass is short. A
    // failed activation doesn't abort the remaining hosts.
    for (i, hostname) in to_activate.iter().enumerate() {
        if i > 0 {
            println!();
        }
        println!("{}", result_header(hostname, &switch_command));
        match executor::run_interactive(hostname, &switch_command) {
            Ok(()) => println!("{}", styles::success().apply_to("✓ done")),
            Err(err) => {
                println!("{}", styles::error().apply_to(&err));
                first_error.get_or_insert(err.to_string());
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
