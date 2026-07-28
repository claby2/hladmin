use anyhow::{Result, bail};
use std::process::{Command, ExitStatus, Stdio};
use std::sync::mpsc;
use std::thread;
use std::time::{Duration, Instant};

/// Result of command execution on a single host.
pub struct ExecResult {
    pub hostname: String,
    pub command: String,
    pub stdout: String,
    pub stderr: String,
    pub err: Option<String>,
    pub duration: Duration,
}

/// Validates that at least one host and a non-empty command were provided.
pub fn verify_hosts_and_command(hosts: &[String], command: &str) -> Result<()> {
    if hosts.is_empty() {
        bail!("at least one hostname must be specified");
    }
    if command.trim().is_empty() {
        bail!("command cannot be empty");
    }
    Ok(())
}

/// Returns the first error found in results, if any.
pub fn results_error(results: &[ExecResult]) -> Result<()> {
    for result in results {
        if let Some(err) = &result.err {
            bail!("{err}");
        }
    }
    Ok(())
}

/// Describes a child exit the way Go's os/exec does ("exit status N").
pub fn describe_exit(status: ExitStatus) -> String {
    if let Some(code) = status.code() {
        return format!("exit status {code}");
    }
    #[cfg(unix)]
    {
        use std::os::unix::process::ExitStatusExt;
        if let Some(signal) = status.signal() {
            return format!("signal: {signal}");
        }
    }
    "exit status unknown".to_string()
}

fn base_command(hostname: &str, command: &str, interactive: bool) -> Command {
    if hostname == "localhost" {
        let mut cmd = Command::new("bash");
        cmd.arg("-c").arg(command);
        cmd
    } else {
        let mut cmd = Command::new("ssh");
        if interactive {
            cmd.arg("-t");
        }
        cmd.arg(hostname).arg(command);
        cmd
    }
}

/// Executes command on a single host, capturing its output. Uses a local bash
/// shell for localhost and SSH for remote hosts.
pub fn run_on_host(hostname: &str, command: &str) -> ExecResult {
    let start = Instant::now();
    let output = base_command(hostname, command, false)
        .stdin(Stdio::null())
        .output();

    match output {
        Ok(output) => ExecResult {
            hostname: hostname.to_string(),
            command: command.to_string(),
            stdout: String::from_utf8_lossy(&output.stdout).into_owned(),
            stderr: String::from_utf8_lossy(&output.stderr).into_owned(),
            err: (!output.status.success()).then(|| {
                format!(
                    "error executing on {hostname}: {}",
                    describe_exit(output.status)
                )
            }),
            duration: start.elapsed(),
        },
        Err(e) => ExecResult {
            hostname: hostname.to_string(),
            command: command.to_string(),
            stdout: String::new(),
            stderr: String::new(),
            err: Some(format!("error executing on {hostname}: {e}")),
            duration: start.elapsed(),
        },
    }
}

/// Executes command on a single host with stdin/stdout/stderr wired to the
/// current process. Presentation is the caller's responsibility.
pub fn run_interactive(hostname: &str, command: &str) -> Result<()> {
    let status = base_command(hostname, command, true)
        .stdin(Stdio::inherit())
        .stdout(Stdio::inherit())
        .stderr(Stdio::inherit())
        .status();

    match status {
        Ok(status) if status.success() => Ok(()),
        Ok(status) => bail!("error executing on {hostname}: {}", describe_exit(status)),
        Err(e) => bail!("error executing on {hostname}: {e}"),
    }
}

/// Executes command on every host concurrently, one thread per host. Each
/// completion is reported on the returned channel as (input index, result).
pub fn run_parallel(hosts: &[String], command: &str) -> mpsc::Receiver<(usize, ExecResult)> {
    let (tx, rx) = mpsc::channel();
    for (i, host) in hosts.iter().enumerate() {
        let tx = tx.clone();
        let host = host.clone();
        let command = command.to_string();
        thread::spawn(move || {
            let result = run_on_host(&host, &command);
            let _ = tx.send((i, result));
        });
    }
    rx
}
