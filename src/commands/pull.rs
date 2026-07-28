use crate::commands::{resolve_hosts, stream_command};
use anyhow::Result;

pub fn run(hosts: &[String]) -> Result<()> {
    let hostnames = resolve_hosts(hosts)?;
    stream_command(&hostnames, "cd $HOME/nix-config && git pull")
}
