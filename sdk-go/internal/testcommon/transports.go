package testcommon

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
)

type transportReplayStart struct {
	BaseURL string `json:"base_url"`
}

type transportReplayVerification struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

type transportReplay struct {
	cmd      *exec.Cmd
	stdout   *bufio.Scanner
	stderr   bytes.Buffer
	finished bool
}

func transportServerPath() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to locate sdk-tests transport server")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "..", "sdk-tests", "transport-server.mjs")
}

func startTransportReplay(testCaseName string) (*transportReplay, string, error) {
	fixtureDir := filepath.Dir(transportServerPath())
	for _, name := range []string{"transport-server.mjs", "protocol.ts", "transports.json", "tests.json"} {
		if _, err := os.ReadFile(filepath.Join(fixtureDir, name)); err != nil {
			return nil, "", fmt.Errorf("read transport fixture dependency %s: %w", name, err)
		}
	}
	cmd := exec.Command("node", transportServerPath(), testCaseName)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, "", fmt.Errorf("open transport replay stdout: %w", err)
	}
	replay := &transportReplay{cmd: cmd, stdout: bufio.NewScanner(stdout)}
	cmd.Stderr = &replay.stderr
	if err := cmd.Start(); err != nil {
		return nil, "", fmt.Errorf("start transport replay: %w", err)
	}
	if !replay.stdout.Scan() {
		_ = cmd.Wait()
		return nil, "", fmt.Errorf("transport replay exited before startup: %s", replay.stderr.String())
	}
	var start transportReplayStart
	if err := json.Unmarshal(replay.stdout.Bytes(), &start); err != nil {
		replay.close()
		return nil, "", fmt.Errorf("decode transport replay startup: %w", err)
	}
	return replay, start.BaseURL, nil
}

func (r *transportReplay) verify() error {
	if !r.stdout.Scan() {
		err := r.cmd.Wait()
		r.finished = true
		return fmt.Errorf("transport replay exited without verification: %v: %s", err, r.stderr.String())
	}
	var verification transportReplayVerification
	if err := json.Unmarshal(r.stdout.Bytes(), &verification); err != nil {
		r.close()
		return fmt.Errorf("decode transport replay verification: %w", err)
	}
	err := r.cmd.Wait()
	r.finished = true
	if err != nil {
		return fmt.Errorf("transport replay failed: %w: %s", err, r.stderr.String())
	}
	if !verification.OK {
		return fmt.Errorf("transport request validation failed: %s", verification.Error)
	}
	return nil
}

func (r *transportReplay) close() {
	if r == nil || r.finished {
		return
	}
	if r.cmd.Process != nil {
		_ = r.cmd.Process.Kill()
	}
	_ = r.cmd.Wait()
	r.finished = true
}

// RunTransportTestGroup replays provider wire fixtures through a model.
func RunTransportTestGroup(t *testing.T, group string, createModel func(baseURL string) llmsdk.LanguageModel) {
	t.Helper()
	testCases, err := getTestCasesByGroup(group)
	if err != nil {
		t.Fatal(err)
	}
	for _, testCaseName := range testCases {
		t.Run(testCaseName, func(t *testing.T) {
			replay, baseURL, err := startTransportReplay(testCaseName)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(replay.close)
			RunTestCase(t, createModel(baseURL), testCaseName)
			if err := replay.verify(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
