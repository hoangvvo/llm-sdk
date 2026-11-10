import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import {
  chmodSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test, { type TestContext } from "node:test";
import {
  mergedRelease,
  prepareRelease,
  releaseInfo,
  releaseRequest,
  verifyRelease,
  type ReleasePR,
} from "./release.ts";

function git(root: string, ...args: string[]): string {
  return execFileSync("git", args, {
    cwd: root,
    encoding: "utf8",
    stdio: ["ignore", "pipe", "pipe"],
  }).trim();
}
function write(root: string, path: string, text: string): void {
  mkdirSync(dirname(join(root, path)), { recursive: true });
  writeFileSync(join(root, path), text);
}
function fixture(t: TestContext): string {
  const root = mkdtempSync(join(tmpdir(), "release-tests-"));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  write(
    root,
    "sdk-js/package.json",
    JSON.stringify({ name: "@hoangvvo/llm-sdk", version: "0.6.0" }),
  );
  write(
    root,
    "agent-js/package.json",
    JSON.stringify({
      name: "@hoangvvo/llm-agent",
      version: "0.3.0",
      dependencies: { "@hoangvvo/llm-sdk": ">=0.5.0 <1.0" },
    }),
  );
  write(
    root,
    "sdk-rust/Cargo.toml",
    '[package]\nname = "llm-sdk-rs"\nversion = "0.4.0"\n',
  );
  write(
    root,
    "agent-rust/Cargo.toml",
    '[package]\nname = "llm-agent"\nversion = "0.3.0"\n\n[dependencies]\nllm-sdk-rs = { version = ">=0.3, <1.0", path = "../sdk-rust", default-features = false }\n',
  );
  write(
    root,
    "sdk-go/go.mod",
    "module github.com/hoangvvo/llm-sdk/sdk-go\n\ngo 1.25.0\n",
  );
  write(
    root,
    "agent-go/go.mod",
    "module github.com/hoangvvo/llm-sdk/agent-go\n\ngo 1.25.0\n\nrequire github.com/hoangvvo/llm-sdk/sdk-go v0.3.0\n",
  );
  git(root, "init", "--initial-branch=main");
  git(root, "config", "user.name", "Test User");
  git(root, "config", "user.email", "test@example.com");
  git(root, "config", "tag.gpgSign", "false");
  git(root, "config", "commit.gpgSign", "false");
  git(root, "add", ".");
  git(root, "commit", "-m", "initial");
  for (const tag of [
    "sdk-js/v0.6.0",
    "sdk-rust/v0.4.0",
    "sdk-go/v0.4.0",
    "agent-go/v0.3.0",
  ])
    git(root, "tag", tag);
  git(root, "update-ref", "refs/remotes/origin/main", "HEAD");
  return root;
}
function releasePR(commit: string): ReleasePR {
  return {
    merged_at: "2026-11-10T00:00:00Z",
    merge_commit_sha: commit,
    base: { ref: "main" },
    head: { ref: "release-sdk-js-v0.7.0", repo: { full_name: "owner/repo" } },
  };
}
function prepared(t: TestContext) {
  const root = fixture(t),
    release = releaseRequest("sdk-js", "0.7.0");
  prepareRelease(root, release);
  git(root, "add", ".");
  git(root, "commit", "-m", "release");
  const commit = git(root, "rev-parse", "HEAD");
  git(root, "update-ref", "refs/remotes/origin/main", commit);
  return { root, release, commit };
}

test("rejects invalid modules and versions before changing files", () => {
  for (const [module, version] of [
    ["../sdk-js", "0.7.0"],
    ["sdk-js", "01.0.0"],
    ["sdk-js", "1.0.0; echo bad"],
    ["sdk-js", "1.0.0-beta.1"],
    ["sdk-js", "99999999999999999.0.0"],
  ]) {
    assert.throws(() => releaseRequest(module ?? "", version ?? ""));
  }
});

for (const [module, version] of [
  ["sdk-js", "0.7.0"],
  ["agent-js", "0.4.0"],
  ["sdk-rust", "0.5.0"],
  ["agent-rust", "0.4.0"],
  ["sdk-go", "0.5.0"],
  ["agent-go", "0.4.0"],
]) {
  test(`prepares ${module} without committing or tagging`, (t) => {
    const root = fixture(t),
      before = git(root, "rev-parse", "HEAD");
    const release = releaseRequest(module ?? "", version ?? "");
    prepareRelease(root, release);
    assert.deepEqual(
      JSON.parse(
        readFileSync(join(root, `.github/releases/${module}.json`), "utf8"),
      ),
      release,
    );
    assert.equal(git(root, "rev-parse", "HEAD"), before);
    assert.equal(git(root, "tag", "--list", releaseInfo(release).tag), "");
    if (module?.endsWith("-js")) {
      const pkg = JSON.parse(
        readFileSync(join(root, module, "package.json"), "utf8"),
      ) as { version: string; dependencies?: Record<string, string> };
      assert.equal(pkg.version, version);
      if (module === "agent-js")
        assert.equal(pkg.dependencies?.["@hoangvvo/llm-sdk"], ">=0.6.0 <1.0");
    } else if (module?.endsWith("-rust")) {
      const manifest = readFileSync(join(root, module, "Cargo.toml"), "utf8");
      assert.ok(manifest.includes(`version = "${version}"`));
      if (module === "agent-rust")
        assert.match(
          manifest,
          /llm-sdk-rs = \{ version = ">=0\.4\.0, <1\.0", path/,
        );
    } else if (module === "agent-go") {
      assert.match(
        readFileSync(join(root, module, "go.mod"), "utf8"),
        /sdk-go v0\.4\.0/,
      );
    }
  });
}

test("rejects downgrades, existing tags, and unpublished SDK changes", (t) => {
  const root = fixture(t);
  assert.throws(
    () => prepareRelease(root, releaseRequest("sdk-js", "0.5.0")),
    /newer/,
  );
  git(root, "tag", "sdk-js/v0.7.0");
  assert.throws(
    () => prepareRelease(root, releaseRequest("sdk-js", "0.7.0")),
    /already exists/,
  );
  write(root, "sdk-js/src/index.ts", "export const changed = true;\n");
  git(root, "add", ".");
  git(root, "commit", "-m", "SDK changes");
  assert.throws(
    () => prepareRelease(root, releaseRequest("agent-js", "0.4.0")),
    /Release sdk-js changes/,
  );
  assert.equal(
    readFileSync(join(root, "agent-js/package.json"), "utf8").includes(
      '"version":"0.3.0"',
    ),
    true,
  );
});

test("only recognizes merged release PRs from this repository to main", () => {
  const pr = releasePR("a".repeat(40));
  assert.deepEqual(mergedRelease(pr, "owner/repo"), {
    module: "sdk-js",
    version: "0.7.0",
    commit: "a".repeat(40),
  });
  assert.equal(
    mergedRelease({ ...pr, merged_at: null }, "owner/repo"),
    undefined,
  );
  assert.equal(
    mergedRelease({ ...pr, base: { ref: "other" } }, "owner/repo"),
    undefined,
  );
  assert.equal(mergedRelease(pr, "another/repository"), undefined);
  assert.equal(
    mergedRelease(
      { ...pr, head: { ...pr.head, ref: "feature" } },
      "owner/repo",
    ),
    undefined,
  );
});

test("publishes only a merged release's exact commit and plan", (t) => {
  const { root, release, commit } = prepared(t),
    pr = releasePR(commit);
  assert.doesNotThrow(() =>
    verifyRelease(root, release, commit, "owner/repo", [pr]),
  );
  git(root, "tag", releaseInfo(release).tag, commit);
  assert.doesNotThrow(() =>
    verifyRelease(root, release, commit, "owner/repo", [pr]),
  );
  assert.throws(
    () => verifyRelease(root, release, "main", "owner/repo", [pr]),
    /full commit SHA/,
  );
  assert.throws(
    () => verifyRelease(root, release, commit, "owner/repo", []),
    /merged release PR/,
  );
  assert.throws(
    () =>
      verifyRelease(root, release, commit, "owner/repo", [
        { ...pr, merged_at: null },
      ]),
    /merged release PR/,
  );
  assert.throws(
    () =>
      verifyRelease(
        root,
        releaseRequest("sdk-js", "0.8.0"),
        commit,
        "owner/repo",
        [{ ...pr, head: { ...pr.head, ref: "release-sdk-js-v0.8.0" } }],
      ),
    /merged plan/,
  );
});

test("rejects a changed manifest or a tag for another commit", (t) => {
  const { root, release, commit } = prepared(t);
  git(root, "tag", releaseInfo(release).tag, "HEAD^");
  assert.throws(
    () =>
      verifyRelease(root, release, commit, "owner/repo", [releasePR(commit)]),
    /another commit/,
  );
  git(root, "tag", "--delete", releaseInfo(release).tag);
  write(root, "sdk-js/package.json", '{"version":"0.8.0"}\n');
  git(root, "add", ".");
  git(root, "commit", "-m", "changed manifest");
  const next = git(root, "rev-parse", "HEAD");
  git(root, "update-ref", "refs/remotes/origin/main", next);
  assert.throws(
    () => verifyRelease(root, release, next, "owner/repo", [releasePR(next)]),
    /Package version/,
  );
  git(root, "update-ref", "refs/remotes/origin/main", commit);
  assert.throws(() =>
    verifyRelease(root, release, next, "owner/repo", [releasePR(next)]),
  );
});

test("retries publication using the original merged PR and commit", (t) => {
  const { root, commit } = prepared(t),
    pr = releasePR(commit);
  const gh = join(root, "bin", "gh");
  write(
    root,
    "bin/gh",
    `#!/usr/bin/env node\nprocess.stdout.write(${JSON.stringify(JSON.stringify([[pr]]))});\n`,
  );
  chmodSync(gh, 0o755);
  const retry = (version: string, releaseCommit: string) =>
    execFileSync(
      process.execPath,
      [fileURLToPath(new URL("./release.ts", import.meta.url)), "retry"],
      {
        cwd: root,
        encoding: "utf8",
        stdio: ["ignore", "pipe", "pipe"],
        env: {
          ...process.env,
          PATH: `${join(root, "bin")}:${process.env["PATH"] ?? ""}`,
          GITHUB_OUTPUT: "",
          GITHUB_REPOSITORY: "owner/repo",
          RELEASE_MODULE: "sdk-js",
          RELEASE_VERSION: version,
          RELEASE_COMMIT: releaseCommit,
        },
      },
    );
  assert.match(
    retry("0.7.0", commit),
    new RegExp(`ready=true\\ncommit=${commit}`),
  );
  assert.throws(() => retry("0.7.0", "main"), /full commit SHA/);
  assert.throws(() => retry("0.8.0", commit), /merged release PR/);
  write(root, "bin/gh", '#!/usr/bin/env node\nprocess.stdout.write("[[]]");\n');
  assert.throws(() => retry("0.7.0", commit), /merged release PR/);
});
