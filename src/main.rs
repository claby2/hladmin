mod cli;
mod commands;
mod config;
mod executor;
mod ui;

use clap::Parser;

fn main() {
    ui::styles::init();
    let cli = cli::Cli::parse();
    if let Err(err) = commands::dispatch(cli.command) {
        eprintln!("Error: {err}");
        std::process::exit(1);
    }
}
