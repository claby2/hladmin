use crate::executor::ExecResult;
use crate::ui::styles;
use std::time::Duration;

/// Renders a duration compactly, e.g. "4.2s" or "1m03s".
pub fn format_duration(d: Duration) -> String {
    if d < Duration::from_secs(60) {
        return format!("{:.1}s", d.as_secs_f64());
    }
    let total = d.as_secs();
    format!("{}m{:02}s", total / 60, total % 60)
}

/// Renders the "===» host" / "cmd: ..." banner for a host.
pub fn result_header(hostname: &str, command: &str) -> String {
    format!(
        "{} {}\n{} {}",
        styles::header().apply_to("===»"),
        styles::hostname().apply_to(hostname),
        styles::secondary().apply_to("cmd:"),
        command
    )
}

/// Renders output with a per-line "host |" prefix, without a trailing newline.
fn prefixed_output(hostname: &str, output: &str) -> String {
    let prefix = format!(
        "{} {} ",
        styles::hostname().apply_to(hostname),
        styles::secondary().apply_to("|")
    );
    let lines: Vec<&str> = output.split('\n').collect();
    let mut out = String::new();
    for (i, line) in lines.iter().enumerate() {
        if i == lines.len() - 1 && line.is_empty() {
            break;
        }
        if !out.is_empty() {
            out.push('\n');
        }
        out.push_str(&prefix);
        out.push_str(line);
    }
    out
}

/// Renders a host's full output block: header, prefixed stdout/stderr, and a
/// colored done/error footer with the duration.
pub fn render_result_block(result: &ExecResult) -> String {
    let mut out = result_header(&result.hostname, &result.command);

    if !result.stdout.is_empty() {
        out.push('\n');
        out.push_str(&prefixed_output(&result.hostname, &result.stdout));
    }
    if !result.stderr.is_empty() {
        out.push('\n');
        out.push_str(&prefixed_output(&result.hostname, &result.stderr));
    }

    let dur = styles::secondary().apply_to(format_duration(result.duration));
    out.push('\n');
    match &result.err {
        Some(err) => out.push_str(&format!("{} {}", styles::error().apply_to(err), dur)),
        None => out.push_str(&format!("{} {}", styles::success().apply_to("✓ done"), dur)),
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn format_duration_sub_minute() {
        assert_eq!(format_duration(Duration::from_millis(4230)), "4.2s");
        assert_eq!(format_duration(Duration::from_millis(0)), "0.0s");
        assert_eq!(format_duration(Duration::from_millis(59900)), "59.9s");
    }

    #[test]
    fn format_duration_minutes() {
        assert_eq!(format_duration(Duration::from_secs(60)), "1m00s");
        assert_eq!(format_duration(Duration::from_secs(63)), "1m03s");
        assert_eq!(format_duration(Duration::from_secs(754)), "12m34s");
    }

    #[test]
    fn prefixed_output_drops_trailing_newline() {
        console::set_colors_enabled(false);
        assert_eq!(prefixed_output("h", "a\nb\n"), "h | a\nh | b");
        assert_eq!(prefixed_output("h", "a"), "h | a");
        assert_eq!(prefixed_output("h", ""), "");
    }
}
