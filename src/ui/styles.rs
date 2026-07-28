use console::Style;
use std::io::IsTerminal;

/// Semantic styles used across the CLI (ANSI 16-color palette).
pub fn success() -> Style {
    Style::new().green()
}
pub fn error() -> Style {
    Style::new().red()
}
pub fn warning() -> Style {
    Style::new().yellow()
}
pub fn info() -> Style {
    Style::new().cyan()
}
pub fn bold() -> Style {
    Style::new().bold()
}
pub fn header() -> Style {
    Style::new().cyan().bold()
}
pub fn hostname() -> Style {
    Style::new().yellow().bold()
}
pub fn secondary() -> Style {
    Style::new().black().bright()
}

/// Decides color output once at startup: colors are emitted only when stdout is
/// a TTY and neither NO_COLOR (the standard) nor HLADMIN_NO_COLOR (tool-specific
/// override) is set to a non-empty value.
pub fn init() {
    let disabled_by_env = |name: &str| std::env::var_os(name).is_some_and(|v| !v.is_empty());
    let enabled = std::io::stdout().is_terminal()
        && !disabled_by_env("NO_COLOR")
        && !disabled_by_env("HLADMIN_NO_COLOR");
    console::set_colors_enabled(enabled);
}

/// Reports whether stdout is connected to a terminal.
pub fn is_terminal() -> bool {
    std::io::stdout().is_terminal()
}
