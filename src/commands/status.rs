use crate::commands::resolve_hosts;
use crate::executor::{self, ExecResult};
use crate::hostid;
use crate::ui::livetable::{TableSpec, run_live_table};
use crate::ui::render::format_duration;
use crate::ui::styles;
use anyhow::Result;

struct HostInfo {
    hostclass: String,
    version: String,
    repo: String,
    disk_usage: String,
    mem_usage: String,
}

const LINUX_MEMORY_COMMAND: &str = r#"free | grep '^Mem:' | awk '{printf "%.0f%%", $3/$2*100}'"#;

const MACOS_MEMORY_COMMAND: &str = r#"vm_stat | awk '
		/^Pages free/ { free = $3 }
		/^Pages inactive/ { inactive = $3 }
		/^Pages wired/ { wired = $3 }
		/^Pages active/ { active = $3 }
		END {
			total = free + inactive + wired + active
			if (total > 0) {
				used = wired + active
				printf "%.0f%%", used/total*100
			} else {
				print "0%"
			}
		}'"#;

fn memory_command() -> String {
    format!(
        "if command -v free >/dev/null 2>&1; then {}; else {}; fi",
        LINUX_MEMORY_COMMAND, MACOS_MEMORY_COMMAND
    )
}

const COMPOUND_PREFIX: &str = r#"
echo -n "$HOSTCLASS|||" && \
echo -n "$(nixos-version --configuration-revision 2>/dev/null || darwin-version --configuration-revision 2>/dev/null || echo 'unknown')|||" && \
echo -n "$(nix flake metadata $HOME/nix-config 2>/dev/null | grep "Revision:" | awk '{print $2}')|||" && \
echo -n "$(df -h / | tail -1 | awk '{print $5}')|||" && \
echo -n "$("#;

const COMPOUND_SUFFIX: &str = r#")" && \
echo
"#;

fn create_compound_status_command() -> String {
    format!("{COMPOUND_PREFIX}{}{COMPOUND_SUFFIX}", memory_command())
}

fn parse_compound_output(output: &str) -> Option<HostInfo> {
    let parts: Vec<&str> = output.trim().split("|||").collect();

    if parts.len() != 5 {
        return None;
    }

    Some(HostInfo {
        hostclass: parts[0].trim().to_string(),
        version: parts[1].trim().to_string(),
        repo: parts[2].trim().to_string(),
        disk_usage: parts[3].trim().to_string(),
        mem_usage: parts[4].trim().to_string(),
    })
}

/// Converts an execution result into the parsed status columns. None means the
/// command failed or produced unparseable output.
fn result_to_host_info(result: &ExecResult) -> Option<HostInfo> {
    if result.err.is_some() {
        return None;
    }
    parse_compound_output(&result.stdout)
}

/// The HOSTNAME cell, marking the machine hladmin is running on. With one
/// config shared fleet-wide the same table is produced from every host, so the
/// marker is what tells you whose view you are looking at. comfy-table measures
/// cell widths ANSI-aware, so the styled marker does not disturb the layout.
fn hostname_cell(hostname: &str) -> String {
    if hostid::is_self(hostname) {
        format!("{hostname} {}", styles::secondary().apply_to("(self)"))
    } else {
        hostname.to_string()
    }
}

/// Defines the status table columns and per-host cell rendering.
fn status_table_spec() -> TableSpec {
    TableSpec {
        headers: vec!["HOSTNAME", "HOSTCLASS", "VERSION", "REPO", "DISK", "MEM"],
        completed_row: Box::new(|result| {
            let Some(info) = result_to_host_info(result) else {
                let error_cell = || styles::error().apply_to("error").to_string();
                let mut row = vec![hostname_cell(&result.hostname)];
                row.resize_with(6, error_cell);
                return row;
            };

            let version_style = if info.version == info.repo {
                styles::success()
            } else {
                styles::warning()
            };

            vec![
                hostname_cell(&result.hostname),
                info.hostclass,
                version_style.apply_to(&info.version).to_string(),
                styles::success().apply_to(&info.repo).to_string(),
                info.disk_usage,
                info.mem_usage,
            ]
        }),
        running_row: Box::new(|host, spinner_frame, elapsed| {
            let indicator = styles::secondary()
                .apply_to(format!(
                    "{spinner_frame} collecting…  {}",
                    format_duration(elapsed)
                ))
                .to_string();
            vec![
                hostname_cell(host),
                indicator,
                String::new(),
                String::new(),
                String::new(),
                String::new(),
            ]
        }),
    }
}

pub fn run(hosts: &[String]) -> Result<()> {
    let hostnames = resolve_hosts(hosts)?;

    let command = create_compound_status_command();

    let results = run_live_table(&hostnames, &command, status_table_spec())?;

    executor::results_error(&results)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_five_fields() {
        let info = parse_compound_output("server|||abc123|||abc123|||42%|||63%\n").unwrap();
        assert_eq!(info.hostclass, "server");
        assert_eq!(info.version, "abc123");
        assert_eq!(info.repo, "abc123");
        assert_eq!(info.disk_usage, "42%");
        assert_eq!(info.mem_usage, "63%");
    }

    #[test]
    fn malformed_output_yields_none() {
        assert!(parse_compound_output("garbage").is_none());
    }

    #[test]
    fn compound_command_shape() {
        let cmd = create_compound_status_command();
        assert!(cmd.starts_with("\necho -n \"$HOSTCLASS|||\""));
        assert!(cmd.contains("nixos-version --configuration-revision"));
        assert!(cmd.contains("command -v free"));
        assert!(cmd.contains("vm_stat"));
        assert!(cmd.ends_with(")\" && \\\necho\n"));
    }
}
