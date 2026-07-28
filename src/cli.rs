use clap::{Parser, Subcommand};

#[derive(Parser)]
#[command(
    name = "hladmin",
    about = "Homelab administration tool",
    arg_required_else_help = true,
    long_about = "A tool for managing homelab servers running NixOS and macOS with nix-darwin"
)]
pub struct Cli {
    #[command(subcommand)]
    pub command: Commands,
}

#[derive(Subcommand)]
pub enum Commands {
    /// Execute command on specified hosts
    #[command(long_about = "Run the specified command with arguments on each host. \
                            Use @group to reference host groups from config.")]
    Exec {
        /// Execute commands with direct stdin/stdout/stderr
        #[arg(short, long)]
        interactive: bool,
        /// Hostnames or @group references
        #[arg(value_name = "HOST")]
        hosts: Vec<String>,
        /// Command and arguments to run (after --)
        #[arg(last = true, value_name = "COMMAND")]
        command: Vec<String>,
    },
    /// Show status information for specified hosts
    #[command(
        long_about = "Display HOSTCLASS, configuration revision, and other useful system \
                            information. Use @group to reference host groups from config."
    )]
    Status {
        /// Hostnames or @group references
        #[arg(value_name = "HOST")]
        hosts: Vec<String>,
    },
    /// Run rebuild script on specified hosts
    #[command(
        long_about = "Execute the rebuild.sh script in $HOME/nix-config on each host. \
                            Use @group to reference host groups from config."
    )]
    Rebuild {
        /// Pass --remote flag to rebuild.sh
        #[arg(long)]
        remote: bool,
        /// Hostnames or @group references
        #[arg(value_name = "HOST")]
        hosts: Vec<String>,
    },
    /// Run git pull on specified hosts
    #[command(long_about = "Execute git pull in $HOME/nix-config on each host. \
                            Use @group to reference host groups from config.")]
    Pull {
        /// Hostnames or @group references
        #[arg(value_name = "HOST")]
        hosts: Vec<String>,
    },
    /// Push staged git changes to specified hosts
    #[command(
        name = "push-staged",
        long_about = "Check for staged changes in $HOME/nix-config and apply them to clean \
                      hosts. Use @group to reference host groups from config."
    )]
    PushStaged {
        /// Show what would be done without making changes
        #[arg(short = 'n', long)]
        dry_run: bool,
        /// Hostnames or @group references
        #[arg(value_name = "HOST")]
        hosts: Vec<String>,
    },
    /// Show host configuration and resolve groups
    #[command(
        long_about = "Show the current host configuration and resolve group references. \
                            Without arguments, displays the full configuration including all \
                            groups and the default group. With arguments, shows how the \
                            specified hosts and groups resolve to individual hostnames. \
                            Use @group to reference host groups from config."
    )]
    Resolve {
        /// Hostnames or @group references
        #[arg(value_name = "HOST")]
        hosts: Vec<String>,
    },
}
