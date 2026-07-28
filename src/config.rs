use anyhow::{Result, bail};
use indexmap::IndexMap;
use std::collections::HashSet;
use std::path::PathBuf;

/// Parsed host configuration: named groups and an optional default group.
#[derive(Debug)]
pub struct HostConfig {
    pub groups: IndexMap<String, Vec<String>>,
    pub default_group: Option<String>,
}

fn config_dir() -> Option<PathBuf> {
    if let Some(xdg) = std::env::var_os("XDG_CONFIG_HOME") {
        if !xdg.is_empty() {
            return Some(PathBuf::from(xdg).join("hladmin"));
        }
    }
    let home = std::env::var_os("HOME")?;
    if home.is_empty() {
        return None;
    }
    Some(PathBuf::from(home).join(".config").join("hladmin"))
}

/// Full path to the hosts config file, if a config directory can be determined.
pub fn config_path() -> Option<PathBuf> {
    Some(config_dir()?.join("hosts"))
}

/// Loads the host configuration from the config file. A missing file yields an
/// empty configuration.
pub fn load_config() -> Result<HostConfig> {
    let empty = HostConfig {
        groups: IndexMap::new(),
        default_group: None,
    };
    let Some(path) = config_path() else {
        return Ok(empty);
    };
    let contents = match std::fs::read_to_string(&path) {
        Ok(c) => c,
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => return Ok(empty),
        Err(e) => bail!("failed to open config file {}: {}", path.display(), e),
    };
    parse_config(&contents)
}

fn parse_config(contents: &str) -> Result<HostConfig> {
    let mut config = HostConfig {
        groups: IndexMap::new(),
        default_group: None,
    };

    for (i, raw_line) in contents.lines().enumerate() {
        let line_num = i + 1;
        let line = raw_line.trim();

        if line.is_empty() || line.starts_with('#') {
            continue;
        }

        let fields: Vec<&str> = line.split_whitespace().collect();
        if fields.len() < 2 {
            bail!("invalid syntax on line {line_num}: {line}");
        }

        match fields[0] {
            "group" => {
                if fields.len() < 3 {
                    bail!("group directive requires at least one host on line {line_num}: {line}");
                }
                let hosts = fields[2..].iter().map(|s| s.to_string()).collect();
                config.groups.insert(fields[1].to_string(), hosts);
            }
            "default" => {
                if fields.len() != 2 {
                    bail!(
                        "default directive requires exactly one group name on line {line_num}: {line}"
                    );
                }
                config.default_group = Some(fields[1].to_string());
            }
            directive => {
                bail!("unknown directive '{directive}' on line {line_num}: {line}");
            }
        }
    }

    if let Some(default) = &config.default_group {
        if !config.groups.contains_key(default) {
            bail!("default group '{default}' is not defined");
        }
    }

    Ok(config)
}

impl HostConfig {
    /// Resolves host arguments (which may include @group syntax) into a flat,
    /// deduplicated list of hostnames. Empty args fall back to the default group
    /// when configured, otherwise an empty list is returned.
    pub fn resolve_hosts(&self, args: &[String]) -> Result<Vec<String>> {
        if args.is_empty() {
            if let Some(default) = &self.default_group {
                if let Some(hosts) = self.groups.get(default) {
                    return Ok(hosts.clone());
                }
            }
            return Ok(Vec::new());
        }

        let mut resolved = Vec::new();
        let mut seen = HashSet::new();

        for arg in args {
            if let Some(group_name) = arg.strip_prefix('@') {
                if group_name.is_empty() {
                    bail!("empty group name: {arg}");
                }
                let Some(hosts) = self.groups.get(group_name) else {
                    bail!("unknown group: {group_name}");
                };
                for host in hosts {
                    if seen.insert(host.clone()) {
                        resolved.push(host.clone());
                    }
                }
            } else if seen.insert(arg.clone()) {
                resolved.push(arg.clone());
            }
        }

        Ok(resolved)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn strs(args: &[&str]) -> Vec<String> {
        args.iter().map(|s| s.to_string()).collect()
    }

    #[test]
    fn parses_groups_and_default() {
        let cfg =
            parse_config("# comment\n\ngroup servers a b c\ngroup desktops d\ndefault servers\n")
                .unwrap();
        assert_eq!(cfg.groups["servers"], strs(&["a", "b", "c"]));
        assert_eq!(cfg.groups["desktops"], strs(&["d"]));
        assert_eq!(cfg.default_group.as_deref(), Some("servers"));
    }

    #[test]
    fn error_invalid_syntax() {
        let err = parse_config("group\n").unwrap_err();
        assert_eq!(err.to_string(), "invalid syntax on line 1: group");
    }

    #[test]
    fn error_group_without_hosts() {
        let err = parse_config("group empty\n").unwrap_err();
        assert_eq!(
            err.to_string(),
            "group directive requires at least one host on line 1: group empty"
        );
    }

    #[test]
    fn error_default_arity() {
        let err = parse_config("group g a\ndefault g extra\n").unwrap_err();
        assert_eq!(
            err.to_string(),
            "default directive requires exactly one group name on line 2: default g extra"
        );
    }

    #[test]
    fn error_unknown_directive() {
        let err = parse_config("bogus thing\n").unwrap_err();
        assert_eq!(
            err.to_string(),
            "unknown directive 'bogus' on line 1: bogus thing"
        );
    }

    #[test]
    fn error_undefined_default() {
        let err = parse_config("default nope\n").unwrap_err();
        assert_eq!(err.to_string(), "default group 'nope' is not defined");
    }

    #[test]
    fn resolves_groups_with_dedup() {
        let cfg = parse_config("group servers a b\ngroup all a c\n").unwrap();
        let hosts = cfg
            .resolve_hosts(&strs(&["@servers", "@all", "b", "d"]))
            .unwrap();
        assert_eq!(hosts, strs(&["a", "b", "c", "d"]));
    }

    #[test]
    fn resolves_default_group_on_empty_args() {
        let cfg = parse_config("group servers a b\ndefault servers\n").unwrap();
        assert_eq!(cfg.resolve_hosts(&[]).unwrap(), strs(&["a", "b"]));
    }

    #[test]
    fn empty_args_without_default_resolves_empty() {
        let cfg = parse_config("group servers a b\n").unwrap();
        assert!(cfg.resolve_hosts(&[]).unwrap().is_empty());
    }

    #[test]
    fn error_unknown_group() {
        let cfg = parse_config("group servers a\n").unwrap();
        let err = cfg.resolve_hosts(&strs(&["@nope"])).unwrap_err();
        assert_eq!(err.to_string(), "unknown group: nope");
    }

    #[test]
    fn error_empty_group_name() {
        let cfg = parse_config("group servers a\n").unwrap();
        let err = cfg.resolve_hosts(&strs(&["@"])).unwrap_err();
        assert_eq!(err.to_string(), "empty group name: @");
    }
}
