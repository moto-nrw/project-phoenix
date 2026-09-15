#!/usr/bin/env node
import { spawnSync } from "node:child_process";
import { existsSync, readFileSync, realpathSync } from "node:fs";
import { dirname, join, delimiter } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(dirname(fileURLToPath(import.meta.url)));
const bin = join(root, ".devbox/nix/profile/default/bin");
let failures = 0;
function check(label, action) {
  try {
    console.log(`PASS ${label}: ${action()}`);
  } catch (error) {
    failures++;
    console.error(`FAIL ${label}: ${error.message}`);
  }
}
function run(command, args) {
  const result = spawnSync(command, args, {
    cwd: root,
    encoding: "utf8",
    timeout: 15000,
  });
  if (result.error || result.status !== 0)
    throw new Error(
      `${command} failed; check installation and run devbox run doctor`,
    );
  return result.stdout.trim();
}
function requireFile(path) {
  if (!existsSync(join(root, path)))
    throw new Error(`${path} missing; run devbox run bootstrap`);
}
for (const [tool, args] of [
  ["go", ["version"]],
  ["node", ["--version"]],
  ["pnpm", ["--version"]],
  ["gopls", ["version"]],
  ["typescript-language-server", ["--version"]],
  ["agent-browser", ["--version"]],
]) {
  check(tool, () => {
    const path = (process.env.PATH ?? "")
      .split(delimiter)
      .map((dir) => join(dir, tool))
      .find(existsSync);
    if (!path || !existsSync(join(bin, tool)))
      throw new Error("not installed; run devbox install");
    if (realpathSync(path) !== realpathSync(join(bin, tool)))
      throw new Error(
        `${path} shadows Devbox; use devbox run or reopen the editor from Devbox`,
      );
    return `${run(path, args).split("\n")[0]} (${path})`;
  });
}
check("Go module/toolchain agreement", () => {
  const expected = readFileSync(join(root, "backend/go.mod"), "utf8").match(
    /^go (\S+)$/m,
  )[1];
  if (run(join(bin, "go"), ["env", "GOVERSION"]) !== `go${expected}`)
    throw new Error("run devbox install to refresh the Go toolchain");
  return expected;
});
check("frontend dependencies", () => {
  for (const path of [
    "frontend/node_modules/typescript/lib/tsserver.js",
    "frontend/node_modules/next/package.json",
    "frontend/node_modules/oxlint/bin/oxlint",
    "frontend/node_modules/prettier/bin/prettier.cjs",
  ])
    requireFile(path);
  return ["typescript", "next", "oxlint", "prettier"]
    .map(
      (name) =>
        `${name} ${JSON.parse(readFileSync(join(root, "frontend/node_modules", name, "package.json"))).version}`,
    )
    .join(", ");
});
check("generated Next types", () => {
  requireFile("frontend/next-env.d.ts");
  requireFile("frontend/.next/types/routes.d.ts");
  return "present";
});
// Agent clients use vendor-managed updates, unlike build tools.
if (
  (process.env.PATH ?? "")
    .split(delimiter)
    .some((dir) => existsSync(join(dir, "claude")))
) {
  check("Claude launcher", () => {
    const version = run(join(root, "scripts/start-claude.sh"), ["--version"]);
    const [major, minor] = version.split(".").map(Number);
    if (major < 2 || (major === 2 && minor < 1))
      throw new Error("update the native Claude installation for LSP support");
    const bare = run("claude", ["--version"]);
    return bare === version
      ? version
      : `${version} (bare claude is ${bare}; use devbox run claude)`;
  });
}
check("Docker Compose", () => run("docker", ["compose", "version", "--short"]));
check("Docker daemon", () =>
  run("docker", ["info", "--format", "{{.ServerVersion}}"]),
);
process.exitCode = failures ? 1 : 0;
