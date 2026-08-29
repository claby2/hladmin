use std::iter::Peekable;
use std::str::Chars;

/// Rebuilds captured command output as the text a terminal would be left
/// showing. Tools like nh/nix emit live-progress control sequences even when
/// piped; replayed verbatim into scrollback they erase our per-line prefixes
/// and corrupt indicatif's draw-region bookkeeping. Carriage returns and
/// in-line erase/column-reset sequences restart the current line (so a
/// progress animation collapses to its final frame) and every other escape
/// sequence, including colors, is stripped.
pub fn sanitize(text: &str) -> String {
    let mut out = String::with_capacity(text.len());
    let mut line = String::new();
    let mut chars = text.chars().peekable();

    while let Some(c) = chars.next() {
        match c {
            '\n' => {
                out.push_str(&line);
                out.push('\n');
                line.clear();
            }
            // CRLF is a plain newline; a lone CR restarts the line, keeping
            // only what the next frame overwrites it with.
            '\r' => {
                if chars.peek() != Some(&'\n') {
                    line.clear();
                }
            }
            '\x08' => {
                line.pop();
            }
            '\x1b' => consume_escape(&mut chars, &mut line),
            c if c.is_control() && c != '\t' => {}
            c => line.push(c),
        }
    }
    out.push_str(&line);
    out
}

/// Consumes one escape sequence following an ESC byte, applying its
/// line-editing effect (if any) to the current line buffer.
fn consume_escape(chars: &mut Peekable<Chars>, line: &mut String) {
    match chars.peek() {
        // CSI: ESC [ <parameter bytes 0x30-0x3F> <intermediates 0x20-0x2F> <final 0x40-0x7E>
        Some('[') => {
            chars.next();
            let mut params = String::new();
            for c in chars.by_ref() {
                match c {
                    '\x30'..='\x3f' => params.push(c),
                    '\x20'..='\x2f' => {}
                    // Cursor-to-column (CHA) and erase-line/erase-left are how
                    // progress bars redraw in place: the line starts over.
                    'G' => return line.clear(),
                    'K' if params.starts_with('1') || params.starts_with('2') => {
                        return line.clear();
                    }
                    _ => return,
                }
            }
        }
        // OSC: ESC ] ... terminated by BEL or ST (ESC \)
        Some(']') => {
            chars.next();
            while let Some(c) = chars.next() {
                match c {
                    '\x07' => return,
                    '\x1b' => {
                        if chars.peek() == Some(&'\\') {
                            chars.next();
                        }
                        return;
                    }
                    _ => {}
                }
            }
        }
        // ESC + intermediate + final (e.g. charset designation ESC ( B)
        Some(&c) if ('\x20'..='\x2f').contains(&c) => {
            chars.next();
            chars.next();
        }
        // Two-byte escape (ESC c, ESC 7, ...)
        Some(_) => {
            chars.next();
        }
        None => {}
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn plain_text_unchanged() {
        assert_eq!(sanitize("hello\nworld\n"), "hello\nworld\n");
        assert_eq!(sanitize("no trailing newline"), "no trailing newline");
        assert_eq!(sanitize("tabs\tkept"), "tabs\tkept");
    }

    #[test]
    fn colors_stripped() {
        assert_eq!(sanitize("\x1b[32m>\x1b[0m Building\n"), "> Building\n");
        assert_eq!(sanitize("\x1b[1;31mbold red\x1b[0m"), "bold red");
    }

    #[test]
    fn carriage_return_keeps_final_frame() {
        assert_eq!(sanitize("frame1\rframe2\rframe3\n"), "frame3\n");
        assert_eq!(sanitize("crlf line\r\nnext"), "crlf line\nnext");
    }

    #[test]
    fn erase_line_redraw_keeps_final_frame() {
        // The redraw idiom nh uses: cursor-to-column-1 + erase-line per frame.
        assert_eq!(
            sanitize("⏱ 0s\x1b[1G\x1b[2K⏱ 1s\x1b[1G\x1b[2K⏱ 2s\n"),
            "⏱ 2s\n"
        );
    }

    #[test]
    fn nh_progress_stream() {
        // Captured verbatim from `nh darwin build` through a pipe.
        let raw = "\x1b[?25l\x1b[?2026h\x1b[1m⏱ 0s\x1b[0m\x1b[?2026l\x1b[?2026h\
                   \x1b[1G\x1b[2K\x1b[1m⏱ 0s\x1b[0m\x1b[?2026l\x1b[?2026h\x1b[1G\x1b[2K\
                   \x1b[1m\x1b[32mFinished at 10:12:33 after 0s\x1b[0m\x1b[0m\x1b[?2026l\x1b[?25h\n";
        assert_eq!(sanitize(raw), "Finished at 10:12:33 after 0s\n");
    }

    #[test]
    fn cursor_movement_and_modes_dropped() {
        assert_eq!(sanitize("a\x1b[2Ab"), "ab");
        assert_eq!(sanitize("\x1b[?25lhidden cursor\x1b[?25h"), "hidden cursor");
        // Erase-to-right at the cursor doesn't clear what came before it.
        assert_eq!(sanitize("kept\x1b[K"), "kept");
        assert_eq!(sanitize("kept\x1b[0K"), "kept");
    }

    #[test]
    fn osc_sequences_dropped() {
        assert_eq!(sanitize("\x1b]0;window title\x07text"), "text");
        assert_eq!(sanitize("\x1b]8;;http://x\x1b\\link"), "link");
    }

    #[test]
    fn backspace_and_charset_escapes() {
        assert_eq!(sanitize("abc\x08d"), "abd");
        assert_eq!(sanitize("\x1b(Btext"), "text");
    }
}
