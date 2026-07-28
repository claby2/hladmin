use crate::executor::{self, ExecResult};
use crate::ui::stream::{SPINNER_FRAMES, TICK_INTERVAL, collect_results};
use crate::ui::styles;
use anyhow::Result;
use comfy_table::{ContentArrangement, Table};
use indicatif::{ProgressBar, ProgressDrawTarget, ProgressStyle};
use std::sync::mpsc::RecvTimeoutError;
use std::time::{Duration, Instant};

/// Describes how to turn per-host execution state into table rows. The caller
/// owns the column semantics and cell styling; this module owns layout and
/// animation.
pub struct TableSpec {
    pub headers: Vec<&'static str>,
    /// Returns the cells for a finished host.
    pub completed_row: CompletedRowFn,
    /// Returns the cells for a host still in progress. `spinner_frame` is the
    /// current spinner glyph.
    pub running_row: RunningRowFn,
}

pub type CompletedRowFn = Box<dyn Fn(&ExecResult) -> Vec<String>>;
pub type RunningRowFn = Box<dyn Fn(&str, &str, Duration) -> Vec<String>>;

/// Lays out headers and rows with a compact, borderless style. When max_width
/// is given, the table is constrained so no line exceeds the terminal width.
fn render_table(spec: &TableSpec, rows: Vec<Vec<String>>, max_width: Option<u16>) -> String {
    let mut table = Table::new();
    table.load_preset(comfy_table::presets::NOTHING);
    table.set_header(spec.headers.clone());
    for row in rows {
        table.add_row(row);
    }

    let last = spec.headers.len() - 1;
    for (i, column) in table.column_iter_mut().enumerate() {
        column.set_padding((0, if i == last { 0 } else { 2 }));
    }

    if let Some(width) = max_width {
        // Dynamic arrangement keeps natural column widths and only shrinks when
        // the table would exceed the terminal width.
        table.set_content_arrangement(ContentArrangement::Dynamic);
        table.set_width(width);
    }

    table.to_string()
}

/// Executes command on hosts in parallel and renders a live table via spec,
/// redrawing in place as hosts complete (TTY). On a non-TTY it renders the
/// final table once after all hosts finish.
pub fn run_live_table(hosts: &[String], command: &str, spec: TableSpec) -> Result<Vec<ExecResult>> {
    executor::verify_hosts_and_command(hosts, command)?;

    let start = Instant::now();
    let rx = executor::run_parallel(hosts, command);

    if !styles::is_terminal() {
        let mut results: Vec<Option<ExecResult>> = hosts.iter().map(|_| None).collect();
        for _ in hosts {
            let Ok((i, result)) = rx.recv() else { break };
            results[i] = Some(result);
        }
        let results = collect_results(results, hosts, command, start);
        let rows = results.iter().map(|r| (spec.completed_row)(r)).collect();
        println!("{}", render_table(&spec, rows, None));
        return Ok(results);
    }

    // A single spinner bar used purely as a multi-line redraw region; indicatif
    // owns the cursor movement and line clearing.
    let pb = ProgressBar::with_draw_target(None, ProgressDrawTarget::stdout());
    pb.set_style(ProgressStyle::with_template("{msg}").expect("valid template"));

    let term_width = console::Term::stdout().size().1;
    let mut results: Vec<Option<ExecResult>> = hosts.iter().map(|_| None).collect();
    let mut remaining = hosts.len();
    let mut frame = 0usize;

    let build_rows = |results: &[Option<ExecResult>], frame: usize| -> Vec<Vec<String>> {
        results
            .iter()
            .enumerate()
            .map(|(i, result)| match result {
                Some(r) => (spec.completed_row)(r),
                None => (spec.running_row)(&hosts[i], SPINNER_FRAMES[frame], start.elapsed()),
            })
            .collect()
    };

    pb.set_message(render_table(
        &spec,
        build_rows(&results, frame),
        Some(term_width),
    ));
    while remaining > 0 {
        match rx.recv_timeout(TICK_INTERVAL) {
            Ok((i, result)) => {
                results[i] = Some(result);
                remaining -= 1;
            }
            Err(RecvTimeoutError::Timeout) => {
                frame = (frame + 1) % SPINNER_FRAMES.len();
            }
            Err(RecvTimeoutError::Disconnected) => break,
        }
        pb.set_message(render_table(
            &spec,
            build_rows(&results, frame),
            Some(term_width),
        ));
    }

    pb.finish_and_clear();
    let results = collect_results(results, hosts, command, start);
    let rows = results.iter().map(|r| (spec.completed_row)(r)).collect();
    println!("{}", render_table(&spec, rows, Some(term_width)));
    Ok(results)
}
