import { execFileSync } from "node:child_process";
import {
  appendFileSync,
  mkdirSync,
  readFileSync,
  writeFileSync,
} from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const modules = {
  "sdk-js": { registry: "npm", package: "@hoangvvo/llm-sdk" },
  "agent-js": { registry: "npm", package: "@hoangvvo/llm-agent" },
  "sdk-rust": { registry: "crates", package: "llm-sdk-rs" },
  "agent-rust": { registry: "crates", package: "llm-agent" },
  "sdk-go": { registry: "go", package: "github.com/hoangvvo/llm-sdk/sdk-go" },
  "agent-go": {
    registry: "go",
    package: "github.com/hoangvvo/llm-sdk/agent-go",
  },
} as const;

export type Module = keyof typeof modules;
export interface Release {
  module: Module;
  version: string;
}
interface Package {
  version: string;
  dependencies?: Record<string, string>;
}
export interface ReleasePR {
  merged_at: string | null;
  merge_commit_sha: string | null;
  base: { ref: string };
  head: { ref: string; repo: { full_name: string } | null };
}

function git(root: string, ...args: string[]): string {
  return execFileSync("git", args, { cwd: root, encoding: "utf8" }).trim();
}
function readJSON<T>(root: string, path: string): T {
  return JSON.parse(readFileSync(resolve(root, path), "utf8")) as T;
}
function writeJSON(root: string, path: string, value: unknown): void {
  const file = resolve(root, path);
  mkdirSync(dirname(file), { recursive: true });
  writeFileSync(file, `${JSON.stringify(value, null, 2)}\n`);
}

export function releaseRequest(module: string, version: string): Release {
  if (!Object.hasOwn(modules, module))
    throw new Error("Unknown release module");
  if (!/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/.test(version)) {
    throw new Error("Version must be in the format X.Y.Z");
  }
  if (version.split(".").some((part) => !Number.isSafeInteger(Number(part)))) {
    throw new Error("Version component is too large");
  }
  return { module: module as Module, version };
}
export function releaseInfo(release: Release) {
  return {
    ...release,
    ...modules[release.module],
    branch: `release-${release.module}-v${release.version}`,
    tag: `${release.module}/v${release.version}`,
  };
}
export function mergedRelease(
  pr: ReleasePR,
  repository: string,
): (Release & { commit: string }) | undefined {
  if (
    !pr.merged_at ||
    !pr.merge_commit_sha ||
    pr.base.ref !== "main" ||
    pr.head.repo?.full_name !== repository
  ) {
    return undefined;
  }
  const match =
    /^release-(sdk-js|agent-js|sdk-rust|agent-rust|sdk-go|agent-go)-v(.+)$/.exec(
      pr.head.ref,
    );
  if (!match?.[1] || !match[2]) return undefined;
  return { ...releaseRequest(match[1], match[2]), commit: pr.merge_commit_sha };
}
function newer(left: string, right: string): boolean {
  const a = left.split(".").map(Number),
    b = right.split(".").map(Number);
  for (let i = 0; i < 3; i++) {
    if (a[i] !== b[i]) return (a[i] ?? 0) > (b[i] ?? 0);
  }
  return false;
}
function latestTag(root: string, module: Module): string | undefined {
  return git(root, "tag", "--list", `${module}/v*`, "--sort=-version:refname")
    .split("\n")
    .find((tag) => new RegExp(`^${module}/v\\d+\\.\\d+\\.\\d+$`).test(tag));
}
function rustVersion(manifest: string): string {
  const section = manifest.split("[package]")[1]?.split("\n[")[0];
  const version = section?.match(/^version\s*=\s*"([^"]+)"/m)?.[1];
  if (!version) throw new Error("Missing Rust package version");
  return version;
}
function packageVersion(root: string, module: Module): string | undefined {
  if (module.endsWith("-js"))
    return readJSON<Package>(root, `${module}/package.json`).version;
  if (module.endsWith("-rust"))
    return rustVersion(
      readFileSync(resolve(root, `${module}/Cargo.toml`), "utf8"),
    );
  return latestTag(root, module)?.split("/v")[1];
}
function sdkVersion(root: string, module: Module): string {
  const sdk = module.replace("agent-", "sdk-") as Module;
  const tag = latestTag(root, sdk);
  if (!tag) throw new Error(`Release ${sdk} before ${module}`);
  const paths = sdk.endsWith("-js")
    ? [`${sdk}/src`, `${sdk}/package.json`]
    : sdk.endsWith("-rust")
      ? [`${sdk}/src`, `${sdk}/Cargo.toml`]
      : [sdk];
  if (git(root, "diff", "--name-only", tag, "HEAD", "--", ...paths)) {
    throw new Error(`Release ${sdk} changes before ${module}`);
  }
  const version = tag.split("/v")[1];
  if (!version) throw new Error("Invalid SDK tag");
  return version;
}

export function prepareRelease(root: string, release: Release): void {
  const current = packageVersion(root, release.module);
  if (current && !newer(release.version, current))
    throw new Error(`Version must be newer than ${current}`);
  if (git(root, "tag", "--list", releaseInfo(release).tag))
    throw new Error("Release tag already exists");
  const sdk = release.module.startsWith("agent-")
    ? sdkVersion(root, release.module)
    : undefined;
  if (release.module.endsWith("-js")) {
    const path = `${release.module}/package.json`,
      pkg = readJSON<Package>(root, path);
    pkg.version = release.version;
    if (sdk) {
      if (!pkg.dependencies) throw new Error("Missing agent dependencies");
      pkg.dependencies["@hoangvvo/llm-sdk"] = `>=${sdk} <1.0`;
    }
    writeJSON(root, path, pkg);
  } else if (release.module.endsWith("-rust")) {
    const path = resolve(root, `${release.module}/Cargo.toml`);
    let manifest = readFileSync(path, "utf8");
    manifest = manifest.replace(
      /^(version\s*=\s*)"[^"]+"/m,
      `$1"${release.version}"`,
    );
    if (sdk) {
      const dependency = /^(llm-sdk-rs\s*=\s*\{[^\n]*?version\s*=\s*)"[^"]+"/m;
      if (!dependency.test(manifest)) throw new Error("Missing SDK dependency");
      manifest = manifest.replace(dependency, `$1">=${sdk}, <1.0"`);
    }
    writeFileSync(path, manifest);
  } else if (sdk) {
    execFileSync(
      "go",
      ["mod", "edit", `-require=${modules["sdk-go"].package}@v${sdk}`],
      {
        cwd: resolve(root, release.module),
        env: { ...process.env, GOWORK: "off" },
      },
    );
  }
  writeJSON(root, `.github/releases/${release.module}.json`, release);
}

export function verifyRelease(
  root: string,
  release: Release,
  commit: string,
  repository: string,
  prs: ReleasePR[],
): void {
  if (!/^[a-f0-9]{40}$/.test(commit))
    throw new Error("A full commit SHA is required");
  git(root, "merge-base", "--is-ancestor", commit, "origin/main");
  if (
    !prs.some((pr) => {
      const merged = mergedRelease(pr, repository);
      return (
        merged?.commit === commit &&
        merged.module === release.module &&
        merged.version === release.version
      );
    })
  )
    throw new Error("Publishing requires a merged release PR");
  const plan = JSON.parse(
    git(root, "show", `${commit}:.github/releases/${release.module}.json`),
  ) as Release;
  if (plan.module !== release.module || plan.version !== release.version)
    throw new Error("Release request does not match the merged plan");
  if (release.module.endsWith("-js")) {
    const pkg = JSON.parse(
      git(root, "show", `${commit}:${release.module}/package.json`),
    ) as Package;
    if (pkg.version !== release.version)
      throw new Error("Package version does not match");
  } else if (release.module.endsWith("-rust")) {
    if (
      rustVersion(
        git(root, "show", `${commit}:${release.module}/Cargo.toml`),
      ) !== release.version
    )
      throw new Error("Package version does not match");
  }
  const tag = releaseInfo(release).tag;
  if (
    git(root, "tag", "--list", tag) &&
    git(root, "rev-parse", `${tag}^{commit}`) !== commit
  )
    throw new Error("Release tag points to another commit");
}
function output(values: Record<string, string>): void {
  const text = Object.entries(values)
    .map(([key, value]) => `${key}=${value}\n`)
    .join("");
  if (process.env["GITHUB_OUTPUT"])
    appendFileSync(process.env["GITHUB_OUTPUT"], text);
  else process.stdout.write(text);
}
function main(): void {
  const root = process.cwd();
  if (process.argv[2] === "merged") {
    const event = readJSON<{ pull_request: ReleasePR }>(
      root,
      process.env["GITHUB_EVENT_PATH"] ?? "",
    );
    const release = mergedRelease(
      event.pull_request,
      process.env["GITHUB_REPOSITORY"] ?? "",
    );
    if (!release) {
      output({ ready: "false" });
      return;
    }
    verifyRelease(
      root,
      release,
      release.commit,
      process.env["GITHUB_REPOSITORY"] ?? "",
      [event.pull_request],
    );
    output({ ready: "true", commit: release.commit, ...releaseInfo(release) });
    return;
  }
  const release = releaseRequest(
    process.env["RELEASE_MODULE"] ?? "",
    process.env["RELEASE_VERSION"] ?? "",
  );
  if (process.argv[2] === "prepare") prepareRelease(root, release);
  else if (process.argv[2] !== "info")
    throw new Error("Unknown release command");
  output(releaseInfo(release));
}
if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  try {
    main();
  } catch (error) {
    process.stderr.write(
      `${error instanceof Error ? error.message : String(error)}\n`,
    );
    process.exitCode = 1;
  }
}
