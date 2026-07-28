use crate::config::{self, HostConfig};
use crate::ui::styles;
use anyhow::Result;

pub fn run(args: &[String]) -> Result<()> {
    let cfg = config::load_config()?;

    let config_path = config::config_path();
    let config_path_display = config_path
        .as_ref()
        .map(|p| p.display().to_string())
        .unwrap_or_else(|| "unknown".to_string());
    let config_exists = config_path.as_ref().is_some_and(|p| p.exists());

    // Always show config location first.
    if config_exists {
        println!(
            "{} {}\n",
            styles::info().apply_to("Config:"),
            config_path_display
        );
    } else {
        println!(
            "{} {} (checked {})\n",
            styles::info().apply_to("Config:"),
            styles::warning().apply_to("No configuration file found"),
            config_path_display
        );
    }

    if args.is_empty() {
        show_full_configuration(&cfg);
        return Ok(());
    }

    show_host_resolution(&cfg, args)
}

fn show_full_configuration(cfg: &HostConfig) {
    if cfg.groups.is_empty() {
        println!("{}", styles::warning().apply_to("No groups defined."));
        return;
    }

    println!("{}", styles::header().apply_to("Groups:"));
    for (group_name, hosts) in &cfg.groups {
        println!(
            "  {}: {}",
            styles::bold().apply_to(format!("@{group_name}")),
            hosts.join(", ")
        );
    }
    println!();

    match &cfg.default_group {
        Some(default) => println!(
            "{} {}",
            styles::info().apply_to("Default Group:"),
            styles::bold().apply_to(default)
        ),
        None => println!(
            "{} {}",
            styles::info().apply_to("Default Group:"),
            styles::secondary().apply_to("none")
        ),
    }
}

fn show_host_resolution(cfg: &HostConfig, args: &[String]) -> Result<()> {
    let resolved_hosts = cfg.resolve_hosts(args)?;

    for arg in args {
        if let Some(group_name) = arg.strip_prefix('@') {
            match cfg.groups.get(group_name) {
                Some(hosts) => println!("{} -> {}", styles::bold().apply_to(arg), hosts.join(", ")),
                None => println!(
                    "{} -> {}",
                    styles::bold().apply_to(arg),
                    styles::error().apply_to("error: unknown group")
                ),
            }
        } else {
            println!("{} -> {}", styles::hostname().apply_to(arg), arg);
        }
    }

    println!();
    println!(
        "{} {}",
        styles::info().apply_to("Final host list:"),
        resolved_hosts.join(", ")
    );
    Ok(())
}
