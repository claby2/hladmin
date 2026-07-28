use crate::commands::{resolve_hosts, stream_command};
use crate::ui;
use anyhow::{Result, bail};

pub fn run(interactive: bool, hosts: &[String], command_args: &[String]) -> Result<()> {
    // clap only fills command_args from tokens after "--", but it cannot
    // distinguish a missing separator from an empty command; check argv.
    if !std::env::args().any(|arg| arg == "--") {
        bail!(
            "command separator '--' not found. Usage: hladmin exec [-i|--interactive] <hosts...> -- <command> [args...]"
        );
    }
    if command_args.is_empty() {
        bail!("no command specified after '--'");
    }
    let command = command_args.join(" ");

    let hostnames = resolve_hosts(hosts)?;

    if interactive {
        return ui::interactive::run_interactive(&hostnames, &command);
    }
    stream_command(&hostnames, &command)
}
