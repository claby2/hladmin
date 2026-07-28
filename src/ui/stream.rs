use crate::executor::{self, ExecResult};
use crate::ui::render::{format_duration, render_result_block};
use crate::ui::styles;
use anyhow::Result;
use indicatif::{MultiProgress, ProgressBar, ProgressDrawTarget, ProgressStyle};
use std::sync::mpsc::RecvTimeoutError;
use std::time::{Duration, Instant};

/// Spinner cadence shared by the streaming and live-table views.
pub const TICK_INTERVAL: Duration = Duration::from_millis(100);

/// Braille dot spinner frames.
pub const SPINNER_FRAMES: [&str; 8] = ["⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"];

/// Executes command on hosts in parallel. On a TTY it shows a live spinner block
/// for running hosts and streams each host's output block into scrollback as it
/// completes. On a non-TTY it prints each block as it finishes with no animation.
pub fn run_streaming(hosts: &[String], command: &str) -> Result<Vec<ExecResult>> {
    executor::verify_hosts_and_command(hosts, command)?;

    if !styles::is_terminal() {
        return Ok(run_streaming_plain(hosts, command));
    }

    let start = Instant::now();
    let rx = executor::run_parallel(hosts, command);

    let mp = MultiProgress::with_draw_target(ProgressDrawTarget::stdout());
    let style = ProgressStyle::with_template("{spinner} {msg}")
        .expect("valid template")
        .tick_strings(&["⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷", "⣷"]);
    let bars: Vec<ProgressBar> = hosts
        .iter()
        .map(|host| {
            let pb = mp.add(ProgressBar::new_spinner());
            pb.set_style(style.clone());
            pb.set_message(running_message(host, start));
            pb
        })
        .collect();

    let mut results: Vec<Option<ExecResult>> = hosts.iter().map(|_| None).collect();
    let mut remaining = hosts.len();
    while remaining > 0 {
        match rx.recv_timeout(TICK_INTERVAL) {
            Ok((i, result)) => {
                bars[i].finish_and_clear();
                let _ = mp.println(render_result_block(&result));
                results[i] = Some(result);
                remaining -= 1;
            }
            Err(RecvTimeoutError::Timeout) => {
                for (i, pb) in bars.iter().enumerate() {
                    if results[i].is_none() {
                        pb.set_message(running_message(&hosts[i], start));
                        pb.tick();
                    }
                }
            }
            Err(RecvTimeoutError::Disconnected) => break,
        }
    }

    Ok(collect_results(results, hosts, command, start))
}

fn running_message(host: &str, start: Instant) -> String {
    format!(
        "{}  {}",
        styles::hostname().apply_to(host),
        styles::secondary().apply_to(format_duration(start.elapsed()))
    )
}

fn run_streaming_plain(hosts: &[String], command: &str) -> Vec<ExecResult> {
    let start = Instant::now();
    let rx = executor::run_parallel(hosts, command);
    let mut results: Vec<Option<ExecResult>> = hosts.iter().map(|_| None).collect();
    for _ in hosts {
        let Ok((i, result)) = rx.recv() else { break };
        println!("{}", render_result_block(&result));
        println!();
        results[i] = Some(result);
    }
    collect_results(results, hosts, command, start)
}

/// Unwraps per-host results, substituting an error result for any host whose
/// worker thread died without reporting (should not happen in practice).
pub fn collect_results(
    results: Vec<Option<ExecResult>>,
    hosts: &[String],
    command: &str,
    start: Instant,
) -> Vec<ExecResult> {
    results
        .into_iter()
        .enumerate()
        .map(|(i, result)| {
            result.unwrap_or_else(|| ExecResult {
                hostname: hosts[i].clone(),
                command: command.to_string(),
                stdout: String::new(),
                stderr: String::new(),
                err: Some(format!(
                    "error executing on {}: worker terminated",
                    hosts[i]
                )),
                duration: start.elapsed(),
            })
        })
        .collect()
}
