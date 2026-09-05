//! Identity of the machine hladmin is running on.
//!
//! Commands name hosts by their real hostnames, so the same hosts file can be
//! byte-identical on every machine in the fleet. Deciding whether a target is
//! *this* machine is therefore a runtime question rather than something the
//! config encodes, and this module is the single place that answers it.

use std::net::IpAddr;
use std::sync::OnceLock;

/// Where [`self_name`] got its answer, for the `resolve` diagnostic.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SelfSource {
    /// Read from `HLADMIN_SELF`.
    Env,
    /// Read from the system hostname.
    Hostname,
    /// Detection failed; nothing is ever local.
    Unknown,
}

/// Dedup key for a host argument. Every spelling of the local machine collapses
/// to [`HostKey::Local`], so a config carrying both `localhost` and the real
/// hostname resolves to a single entry instead of executing twice.
#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub enum HostKey {
    Local,
    Remote(String),
}

struct SelfId {
    name: Option<String>,
    source: SelfSource,
}

static SELF_ID: OnceLock<SelfId> = OnceLock::new();

fn self_id() -> &'static SelfId {
    SELF_ID.get_or_init(detect)
}

fn detect() -> SelfId {
    // An explicitly empty HLADMIN_SELF means "this machine is nobody": nothing
    // is local and every host goes over SSH. That is the escape hatch for
    // debugging, so unset and set-but-empty must stay distinguishable.
    if let Some(value) = std::env::var_os("HLADMIN_SELF") {
        return SelfId {
            name: normalize(&value.to_string_lossy()),
            source: SelfSource::Env,
        };
    }

    let hostname = gethostname::gethostname().to_string_lossy().into_owned();
    match normalize(&hostname) {
        Some(name) => SelfId {
            name: Some(name),
            source: SelfSource::Hostname,
        },
        None => SelfId {
            name: None,
            source: SelfSource::Unknown,
        },
    }
}

/// Canonical name of the machine hladmin runs on, resolved once per process.
/// `None` means detection failed or was disabled, in which case nothing is
/// considered local.
pub fn self_name() -> Option<&'static str> {
    self_id().name.as_deref()
}

/// Where [`self_name`] came from.
pub fn self_source() -> SelfSource {
    self_id().source
}

/// Whether `host` refers to the machine hladmin runs on.
pub fn is_self(host: &str) -> bool {
    // Loopback literals are unambiguous even when detection failed, which keeps
    // `hladmin exec localhost` and un-migrated configs working.
    if is_loopback_literal(host) {
        return true;
    }
    match self_name() {
        Some(name) => matches_self(name, host),
        None => false,
    }
}

/// Dedup key for `host`. See [`HostKey`].
pub fn host_key(host: &str) -> HostKey {
    if is_self(host) {
        HostKey::Local
    } else {
        HostKey::Remote(host.to_string())
    }
}

fn is_loopback_literal(host: &str) -> bool {
    matches!(
        host.to_ascii_lowercase().as_str(),
        "localhost" | "127.0.0.1" | "::1"
    )
}

/// The comparison core, split out so tests can exercise it without touching the
/// process environment.
///
/// Normalization here is for comparison only — the caller keeps the original
/// spelling for the `ssh` argv, since OpenSSH matches `Host` patterns in
/// `~/.ssh/config` case-sensitively.
fn matches_self(self_name: &str, host: &str) -> bool {
    if self_name.is_empty() || host.is_empty() {
        return false;
    }

    // `root@altaria` on altaria names a different user. Running it through a
    // local shell would silently run as the invoking user instead, so a
    // user-qualified target always stays a remote invocation.
    if host.contains('@') {
        return false;
    }

    let lowered = host.to_ascii_lowercase();
    if lowered == self_name {
        return true;
    }

    // Only the config side is label-split here, and never for an IP literal:
    // splitting both sides would make `a.corp.internal` match `a.example.com`,
    // and would compare `10.0.0.5` as `10`.
    first_label(&lowered).is_some_and(|label| label == self_name)
}

/// The leading DNS label of `value`, or `None` when `value` is an IP literal or
/// has no non-empty first label.
fn first_label(value: &str) -> Option<&str> {
    if value.parse::<IpAddr>().is_ok() {
        return None;
    }
    value.split('.').next().filter(|label| !label.is_empty())
}

/// Lowercases `value` and shortens an FQDN to its first label. `None` for an
/// empty result.
fn normalize(value: &str) -> Option<String> {
    let lowered = value.trim().to_ascii_lowercase();
    if lowered.is_empty() {
        return None;
    }
    Some(first_label(&lowered).unwrap_or(&lowered).to_string())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn normalizes_case_and_fqdn() {
        assert_eq!(normalize("Machamp.local").as_deref(), Some("machamp"));
        assert_eq!(normalize("ALTARIA").as_deref(), Some("altaria"));
        assert_eq!(normalize("  onix  ").as_deref(), Some("onix"));
    }

    #[test]
    fn normalize_keeps_ip_literals_whole() {
        assert_eq!(normalize("10.0.0.5").as_deref(), Some("10.0.0.5"));
        assert_eq!(normalize("::1").as_deref(), Some("::1"));
    }

    #[test]
    fn normalize_rejects_empty() {
        assert_eq!(normalize(""), None);
        assert_eq!(normalize("   "), None);
    }

    #[test]
    fn matches_exact_and_case_insensitively() {
        assert!(matches_self("altaria", "altaria"));
        assert!(matches_self("altaria", "ALTARIA"));
        assert!(!matches_self("altaria", "onix"));
    }

    #[test]
    fn matches_fqdn_against_short_self() {
        assert!(matches_self("machamp", "machamp.local"));
        assert!(matches_self("machamp", "Machamp.LAN"));
    }

    #[test]
    fn does_not_match_across_different_domains() {
        // Splitting both sides would collapse these to the same label.
        assert!(!matches_self("a.corp.internal", "a.example.com"));
    }

    #[test]
    fn never_label_splits_ip_literals() {
        assert!(!matches_self("10", "10.0.0.5"));
        assert!(!matches_self("192", "192.168.1.7"));
        assert!(matches_self("10.0.0.5", "10.0.0.5"));
    }

    #[test]
    fn user_qualified_targets_are_never_self() {
        assert!(!matches_self("altaria", "root@altaria"));
        assert!(!matches_self("altaria", "me@altaria.lan"));
    }

    #[test]
    fn empty_sides_never_match() {
        assert!(!matches_self("", "altaria"));
        assert!(!matches_self("altaria", ""));
        assert!(!matches_self("altaria", "."));
    }

    #[test]
    fn loopback_literals_are_self() {
        assert!(is_loopback_literal("localhost"));
        assert!(is_loopback_literal("LocalHost"));
        assert!(is_loopback_literal("127.0.0.1"));
        assert!(is_loopback_literal("::1"));
        assert!(!is_loopback_literal("altaria"));
        assert!(!is_loopback_literal("localhost.example.com"));
    }

    #[test]
    fn first_label_behavior() {
        assert_eq!(first_label("machamp.local"), Some("machamp"));
        assert_eq!(first_label("machamp"), Some("machamp"));
        assert_eq!(first_label("10.0.0.5"), None);
        assert_eq!(first_label(".leading"), None);
    }
}
