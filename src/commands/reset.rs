use crate::commands::{resolve_hosts, stream_command};
use crate::executor::{self, ExecResult};
use crate::hostid;
use crate::ui::styles;
use anyhow::{Result, bail};
use std::io::{BufRead, IsTerminal, Write};

/// Detects work a hard reset would destroy: the branch header reports unpushed
/// commits, the remaining lines are dirty working-tree entries.
const CHECK_COMMAND: &str = "cd $HOME/nix-config && git status --porcelain=v1 -b";

/// `checkout -f` handles a dirty tree on any branch, the reset pins main to
/// origin/main, and `clean -fd` removes untracked files (but not ignored ones).
const RESET_COMMAND: &str = "cd $HOME/nix-config && git fetch origin && git checkout -f main \
                             && git reset --hard origin/main && git clean -fd";

/// Work on a host that a hard reset would permanently destroy.
struct PendingWork {
    /// Commits ahead of the upstream branch.
    ahead: usize,
    /// Dirty working-tree entries (modified/staged/untracked), porcelain lines.
    dirty: Vec<String>,
}

impl PendingWork {
    fn loses_work(&self) -> bool {
        self.ahead > 0 || !self.dirty.is_empty()
    }
}

/// Parses `git status --porcelain=v1 -b` output: the `## ` header line yields
/// the ahead count (e.g. `## main...origin/main [ahead 2]`), every other
/// non-empty line is a dirty entry.
fn parse_porcelain_branch(output: &str) -> PendingWork {
    let mut ahead = 0;
    let mut dirty = Vec::new();
    for line in output.lines() {
        if let Some(branch_info) = line.strip_prefix("## ") {
            if let Some((_, rest)) = branch_info.split_once("[ahead ") {
                let digits: String = rest.chars().take_while(|c| c.is_ascii_digit()).collect();
                ahead = digits.parse().unwrap_or(0);
            }
        } else if !line.trim().is_empty() {
            dirty.push(line.to_string());
        }
    }
    PendingWork { ahead, dirty }
}

/// Asks the user to confirm the reset. Requires an interactive stdin so a
/// destructive default cannot slip through a pipeline.
fn confirm() -> Result<bool> {
    if !std::io::stdin().is_terminal() {
        bail!("confirmation required but stdin is not a terminal; rerun with --yes");
    }
    print!("Continue? [y/N] ");
    std::io::stdout().flush()?;
    let mut line = String::new();
    std::io::stdin().lock().read_line(&mut line)?;
    let answer = line.trim();
    Ok(answer.eq_ignore_ascii_case("y") || answer.eq_ignore_ascii_case("yes"))
}

pub fn run(yes: bool, hosts: &[String]) -> Result<()> {
    let hostnames = resolve_hosts(hosts)?;

    let (local, remote): (Vec<String>, Vec<String>) =
        hostnames.into_iter().partition(|h| hostid::is_self(h));
    if !local.is_empty() {
        println!(
            "{}",
            styles::info().apply_to(format!(
                "Skipping {}: reset would discard local work",
                local.join(", ")
            ))
        );
    }
    if remote.is_empty() {
        println!("{}", styles::info().apply_to("No remote hosts to reset"));
        return Ok(());
    }

    // Quiet parallel pre-check; results are re-ordered back to input order so
    // messages and error precedence are deterministic.
    let rx = executor::run_parallel(&remote, CHECK_COMMAND)?;
    let mut results: Vec<Option<ExecResult>> = (0..remote.len()).map(|_| None).collect();
    for (i, result) in rx.iter().take(remote.len()) {
        results[i] = Some(result);
    }

    let mut check_error = None;
    let mut to_reset = Vec::new();
    let mut at_risk = Vec::new();
    for result in results.into_iter().flatten() {
        if let Some(err) = result.err {
            println!(
                "{}",
                styles::error().apply_to(format!("{err}, skipping reset"))
            );
            if !result.stderr.trim().is_empty() {
                println!(
                    "{}",
                    styles::secondary().apply_to(format!("  {}", result.stderr.trim()))
                );
            }
            check_error.get_or_insert(err);
            continue;
        }
        let work = parse_porcelain_branch(&result.stdout);
        if work.loses_work() {
            at_risk.push((result.hostname.clone(), work));
        }
        to_reset.push(result.hostname);
    }

    if !at_risk.is_empty() && !yes {
        println!(
            "{}",
            styles::warning().apply_to("The following will be permanently destroyed:")
        );
        for (hostname, work) in &at_risk {
            println!("{}", styles::hostname().apply_to(hostname));
            if work.ahead > 0 {
                println!(
                    "  {}",
                    styles::warning().apply_to(format!("{} unpushed commit(s)", work.ahead))
                );
            }
            for entry in &work.dirty {
                println!("  {}", styles::secondary().apply_to(entry));
            }
        }
        if !confirm()? {
            println!("Aborted");
            return Ok(());
        }
    }

    let stream_result = if to_reset.is_empty() {
        Ok(())
    } else {
        stream_command(&to_reset, RESET_COMMAND)
    };

    // A failed pre-check must fail the command even when every reset succeeded.
    if let Some(err) = check_error {
        bail!("{err}");
    }
    stream_result
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_clean_repo() {
        let work = parse_porcelain_branch("## main...origin/main\n");
        assert_eq!(work.ahead, 0);
        assert!(work.dirty.is_empty());
        assert!(!work.loses_work());
    }

    #[test]
    fn parse_dirty_entries() {
        let work = parse_porcelain_branch("## main...origin/main\n M flake.nix\n?? new.txt\n");
        assert_eq!(work.ahead, 0);
        assert_eq!(work.dirty, vec![" M flake.nix", "?? new.txt"]);
        assert!(work.loses_work());
    }

    #[test]
    fn parse_ahead_count() {
        let work = parse_porcelain_branch("## main...origin/main [ahead 2]\n");
        assert_eq!(work.ahead, 2);
        assert!(work.dirty.is_empty());
        assert!(work.loses_work());
    }

    #[test]
    fn parse_ahead_behind_and_dirty() {
        let work =
            parse_porcelain_branch("## main...origin/main [ahead 12, behind 3]\n M rebuild.sh\n");
        assert_eq!(work.ahead, 12);
        assert_eq!(work.dirty, vec![" M rebuild.sh"]);
        assert!(work.loses_work());
    }

    #[test]
    fn parse_behind_only_is_not_at_risk() {
        let work = parse_porcelain_branch("## main...origin/main [behind 3]\n");
        assert_eq!(work.ahead, 0);
        assert!(!work.loses_work());
    }

    #[test]
    fn parse_no_upstream() {
        let work = parse_porcelain_branch("## main\n");
        assert_eq!(work.ahead, 0);
        assert!(!work.loses_work());
    }
}
