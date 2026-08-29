use crate::executor::{self, ExecResult};
use crate::ui::render::{format_duration, render_result_block};
use crate::ui::styles;
use anyhow::Result;
use indicatif::{MultiProgress, ProgressBar, ProgressDrawTarget, ProgressStyle};
use std::sync::mpsc::RecvTimeoutError;
use std::time::{Duration, Instant};

/// Spinner cadence shared by the streaming and live-table views.
pub const TICK_INTERVAL: Duration = Duration::from_millis(100);

/// Executes command on hosts in parallel. On a TTY it shows a live spinner block
/// for running hosts and streams each host's output block into scrollback as it
/// completes. On a non-TTY it prints each block as it finishes with no animation.
pub fn run_streaming(hosts: &[String], command: &str) -> Result<Vec<ExecResult>> {
    if !styles::is_terminal() {
        return run_streaming_plain(hosts, command);
    }

    let start = Instant::now();
    let rx = executor::run_parallel(hosts, command)?;

    let mp = MultiProgress::with_draw_target(ProgressDrawTarget::stdout());
    // new_spinner comes preconfigured with indicatif's default spinner style,
    // whose template is exactly the "{spinner} {msg}" layout used here.
    let bars: Vec<ProgressBar> = hosts
        .iter()
        .map(|host| {
            let pb = mp.add(ProgressBar::new_spinner());
            pb.set_message(running_message(host, start));
            pb
        })
        .collect();

    // A single-host run needs no tally; for multiple hosts a summary line
    // below the spinners keeps overall progress visible even once completed
    // blocks start filling the scrollback.
    let summary = (hosts.len() > 1).then(|| {
        let pb = mp.add(ProgressBar::new_spinner());
        pb.set_style(ProgressStyle::with_template("{msg}").expect("valid template"));
        pb
    });
    let update_summary = |done: usize| {
        if let Some(pb) = &summary {
            pb.set_message(summary_message(done, hosts.len(), start));
            pb.tick();
        }
    };
    update_summary(0);

    let mut results: Vec<Option<ExecResult>> = hosts.iter().map(|_| None).collect();
    let mut remaining = hosts.len();
    while remaining > 0 {
        match rx.recv_timeout(TICK_INTERVAL) {
            Ok((i, result)) => {
                bars[i].finish_and_clear();
                let _ = mp.println(render_result_block(&result));
                results[i] = Some(result);
                remaining -= 1;
                update_summary(hosts.len() - remaining);
            }
            Err(RecvTimeoutError::Timeout) => {
                for (i, pb) in bars.iter().enumerate() {
                    if results[i].is_none() {
                        pb.set_message(running_message(&hosts[i], start));
                        pb.tick();
                    }
                }
                update_summary(hosts.len() - remaining);
            }
            Err(RecvTimeoutError::Disconnected) => break,
        }
    }
    if let Some(pb) = &summary {
        pb.finish_and_clear();
    }

    Ok(collect_results(results, hosts, command, start))
}

fn summary_message(done: usize, total: usize, start: Instant) -> String {
    styles::secondary()
        .apply_to(format!(
            "{done}/{total} done · {}",
            format_duration(start.elapsed())
        ))
        .to_string()
}

fn running_message(host: &str, start: Instant) -> String {
    format!(
        "{}  {}",
        styles::hostname().apply_to(host),
        styles::secondary().apply_to(format_duration(start.elapsed()))
    )
}

fn run_streaming_plain(hosts: &[String], command: &str) -> Result<Vec<ExecResult>> {
    let start = Instant::now();
    let rx = executor::run_parallel(hosts, command)?;
    let mut results: Vec<Option<ExecResult>> = hosts.iter().map(|_| None).collect();
    for _ in hosts {
        let Ok((i, result)) = rx.recv() else { break };
        println!("{}", render_result_block(&result));
        println!();
        results[i] = Some(result);
    }
    Ok(collect_results(results, hosts, command, start))
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
