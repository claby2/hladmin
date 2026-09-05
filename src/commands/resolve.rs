use crate::config::{self, HostConfig};
use crate::hostid::{self, SelfSource};
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
            "{} {}",
            styles::info().apply_to("Config:"),
            config_path_display
        );
    } else {
        println!(
            "{} {} (checked {})",
            styles::info().apply_to("Config:"),
            styles::warning().apply_to("No configuration file found"),
            config_path_display
        );
    }

    show_self(&cfg);
    println!();

    if args.is_empty() {
        show_full_configuration(&cfg);
        return Ok(());
    }

    show_host_resolution(&cfg, args)
}

/// Reports which machine hladmin thinks it is running on. Without this the
/// failure mode is silent: a self name that matches nothing in the config just
/// sends every host over SSH, and nothing looks wrong.
fn show_self(cfg: &HostConfig) {
    let Some(name) = hostid::self_name() else {
        let detail = match hostid::self_source() {
            SelfSource::Env => "HLADMIN_SELF is empty; all hosts will use SSH",
            _ => "hostname detection failed; all hosts will use SSH",
        };
        println!(
            "{} {} ({})",
            styles::info().apply_to("Self:"),
            styles::warning().apply_to("none"),
            styles::secondary().apply_to(detail)
        );
        return;
    };

    let source = match hostid::self_source() {
        SelfSource::Env => "HLADMIN_SELF",
        SelfSource::Hostname => "hostname",
        SelfSource::Unknown => "unknown",
    };
    println!(
        "{} {} ({})",
        styles::info().apply_to("Self:"),
        styles::hostname().apply_to(name),
        styles::secondary().apply_to(source)
    );

    if !cfg.groups.is_empty() && !configured_hosts_include_self(cfg) {
        println!(
            "{} this machine is not in the config; all hosts will use SSH",
            styles::warning().apply_to("Warning:")
        );
    }
}

fn configured_hosts_include_self(cfg: &HostConfig) -> bool {
    cfg.groups
        .values()
        .flatten()
        .any(|host| hostid::is_self(host))
}

/// Appends a `(self)` marker to the host that is this machine.
fn label(host: &str) -> String {
    if hostid::is_self(host) {
        format!("{host} {}", styles::secondary().apply_to("(self)"))
    } else {
        host.to_string()
    }
}

fn join_labeled(hosts: &[String]) -> String {
    hosts
        .iter()
        .map(|h| label(h))
        .collect::<Vec<_>>()
        .join(", ")
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
            join_labeled(hosts)
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
                Some(hosts) => {
                    println!(
                        "{} -> {}",
                        styles::bold().apply_to(arg),
                        join_labeled(hosts)
                    )
                }
                None => println!(
                    "{} -> {}",
                    styles::bold().apply_to(arg),
                    styles::error().apply_to("error: unknown group")
                ),
            }
        } else {
            println!("{} -> {}", styles::hostname().apply_to(arg), label(arg));
        }
    }

    println!();
    println!(
        "{} {}",
        styles::info().apply_to("Final host list:"),
        join_labeled(&resolved_hosts)
    );

    // A config mid-migration can name this machine twice (`localhost` plus the
    // real hostname). resolve_hosts already collapsed them, so this counts the
    // pre-dedup expansion — otherwise the entry would vanish with no
    // explanation of where it went.
    if self_mentions(cfg, args) > 1 {
        println!(
            "{} multiple entries name this machine; they were collapsed into one",
            styles::warning().apply_to("Warning:")
        );
    }

    Ok(())
}

/// How many times `args` names this machine, before deduplication.
fn self_mentions(cfg: &HostConfig, args: &[String]) -> usize {
    let mut count = 0;
    for arg in args {
        match arg.strip_prefix('@') {
            Some(group_name) => {
                if let Some(hosts) = cfg.groups.get(group_name) {
                    count += hosts.iter().filter(|h| hostid::is_self(h)).count();
                }
            }
            None => {
                if hostid::is_self(arg) {
                    count += 1;
                }
            }
        }
    }
    count
}
