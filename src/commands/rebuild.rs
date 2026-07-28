use crate::commands::resolve_hosts;
use crate::ui;
use anyhow::Result;

pub fn run(remote: bool, hosts: &[String]) -> Result<()> {
    let hostnames = resolve_hosts(hosts)?;

    let mut command = String::from("cd $HOME/nix-config && ./rebuild.sh");
    if remote {
        command.push_str(" --remote");
    }

    ui::interactive::run_interactive(&hostnames, &command)
}
