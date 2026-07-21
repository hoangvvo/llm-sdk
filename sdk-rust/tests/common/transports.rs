use super::cases::{run_test_case, test_cases_by_group};
use llm_sdk::LanguageModel;
use serde::Deserialize;
use std::{
    error::Error,
    io::{BufRead, BufReader, Read},
    path::PathBuf,
    process::{Child, ChildStderr, ChildStdout, Command, Stdio},
};

#[derive(Deserialize)]
struct ReplayStart {
    base_url: String,
}

#[derive(Deserialize)]
struct ReplayVerification {
    ok: bool,
    error: Option<String>,
}

struct TransportReplay {
    child: Child,
    stdout: BufReader<ChildStdout>,
    stderr: Option<ChildStderr>,
    finished: bool,
}

fn transport_server_path() -> PathBuf {
    let mut path = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    path.push("../sdk-tests/transport-server.mjs");
    path
}

impl TransportReplay {
    fn start(test_case_name: &str) -> Result<(Self, String), Box<dyn Error>> {
        let mut child = Command::new("node")
            .arg(transport_server_path())
            .arg(test_case_name)
            .stdin(Stdio::null())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()?;
        let stdout = child
            .stdout
            .take()
            .ok_or_else(|| std::io::Error::other("missing transport replay stdout"))?;
        let stderr = child.stderr.take();
        let mut replay = Self {
            child,
            stdout: BufReader::new(stdout),
            stderr,
            finished: false,
        };
        let mut line = String::new();
        if replay.stdout.read_line(&mut line)? == 0 {
            return Err(std::io::Error::other("transport replay exited before startup").into());
        }
        let start: ReplayStart = serde_json::from_str(&line)?;
        Ok((replay, start.base_url))
    }

    fn verify(&mut self) -> Result<(), Box<dyn Error>> {
        let mut line = String::new();
        let read = self.stdout.read_line(&mut line)?;
        let status = self.child.wait()?;
        self.finished = true;
        let mut stderr = String::new();
        if let Some(mut stream) = self.stderr.take() {
            stream.read_to_string(&mut stderr)?;
        }
        if read == 0 {
            return Err(std::io::Error::other(format!(
                "transport replay exited without verification ({status}): {stderr}"
            ))
            .into());
        }
        let verification: ReplayVerification = serde_json::from_str(&line)?;
        if !status.success() || !verification.ok {
            return Err(std::io::Error::other(
                verification
                    .error
                    .unwrap_or_else(|| format!("transport replay failed ({status}): {stderr}")),
            )
            .into());
        }
        Ok(())
    }
}

impl Drop for TransportReplay {
    fn drop(&mut self) {
        if self.finished {
            return;
        }
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
}

pub async fn run_transport_test_group<M, F>(
    group: &str,
    create_model: F,
) -> Result<(), Box<dyn Error>>
where
    M: LanguageModel,
    F: Fn(&str) -> M,
{
    let mut failures = Vec::new();
    for test_case_name in test_cases_by_group(group)? {
        let (mut replay, base_url) = TransportReplay::start(&test_case_name)?;
        let model = create_model(&base_url);
        let execution = run_test_case(&model, &test_case_name, None).await;
        let verification = replay.verify();
        if let Err(error) = execution {
            failures.push(format!("transport case {test_case_name:?} failed: {error}"));
        }
        if let Err(error) = verification {
            failures.push(format!(
                "transport request {test_case_name:?} failed: {error}"
            ));
        }
    }
    if failures.is_empty() {
        Ok(())
    } else {
        Err(std::io::Error::other(failures.join("\n")).into())
    }
}
