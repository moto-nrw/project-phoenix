#!/usr/bin/env node
// Exercise real stdio servers with disposable source files. No services or secrets.
import assert from "node:assert/strict";
import { spawn, spawnSync } from "node:child_process";
import { readFileSync, writeFileSync, rmSync, mkdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = dirname(dirname(fileURLToPath(import.meta.url)));
const zed = JSON.parse(readFileSync(join(root, ".zed/settings.json")));
const claude = JSON.parse(
  readFileSync(
    join(root, ".claude/plugins/moto-lsp/.claude-plugin/plugin.json"),
  ),
).lspServers;
const uri = (path) => pathToFileURL(path).href;
const clients = [];
const temporary = [];
const timeout = 60000;
function cleanup() {
  for (const client of clients)
    if (client.process.exitCode === null) client.process.kill();
  for (const path of [...temporary].reverse())
    rmSync(path, { recursive: true, force: true });
}
process.once("exit", cleanup);
process.once("SIGINT", () => process.exit(130));
process.once("SIGTERM", () => process.exit(143));

class Lsp {
  constructor(tool, args, settings = {}) {
    this.pending = new Map();
    this.messages = [];
    this.sequence = 0;
    this.buffer = Buffer.alloc(0);
    this.settings = settings;
    this.process = spawn(
      join(root, "scripts/editor-tool.sh"),
      [tool, ...args],
      { cwd: root, stdio: ["pipe", "pipe", "pipe"] },
    );
    this.process.stderr.on("data", () => {});
    this.process.stdout.on("data", (chunk) => {
      this.buffer = Buffer.concat([this.buffer, chunk]);
      while (true) {
        const split = this.buffer.indexOf("\r\n\r\n");
        if (split < 0) return;
        const length = Number(
          this.buffer
            .subarray(0, split)
            .toString()
            .match(/Content-Length: (\d+)/i)?.[1],
        );
        if (!Number.isFinite(length)) throw new Error("Invalid LSP frame");
        if (this.buffer.length < split + 4 + length) return;
        const message = JSON.parse(
          this.buffer.subarray(split + 4, split + 4 + length),
        );
        this.buffer = this.buffer.subarray(split + 4 + length);
        if (message.method && message.id !== undefined) {
          this.send({
            id: message.id,
            result:
              message.method === "workspace/configuration"
                ? message.params.items.map(() => this.settings)
                : null,
          });
        } else if (message.id !== undefined) {
          const pending = this.pending.get(message.id);
          this.pending.delete(message.id);
          if (message.error)
            pending?.reject(new Error(JSON.stringify(message.error)));
          else pending?.resolve(message.result);
        } else this.messages.push(message);
      }
    });
    const fail = (error) => {
      for (const pending of this.pending.values()) pending.reject(error);
      this.pending.clear();
    };
    this.process.on("error", fail);
    this.process.on("exit", (code) =>
      fail(new Error(`${tool} exited (${code})`)),
    );
    clients.push(this);
  }
  send(message) {
    const body = JSON.stringify({ jsonrpc: "2.0", ...message });
    this.process.stdin.write(
      `Content-Length: ${Buffer.byteLength(body)}\r\n\r\n${body}`,
    );
  }
  notify(method, params) {
    this.send({ method, params });
  }
  request(method, params) {
    const id = ++this.sequence;
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject(new Error(`${method} timed out`));
      }, timeout);
      this.pending.set(id, {
        resolve: (value) => {
          clearTimeout(timer);
          resolve(value);
        },
        reject: (error) => {
          clearTimeout(timer);
          reject(error);
        },
      });
      this.send({ id, method, params });
    });
  }
  async notification(predicate) {
    const deadline = Date.now() + timeout;
    while (Date.now() < deadline) {
      const found = this.messages.find(predicate);
      if (found) return found;
      if (this.process.exitCode !== null)
        throw new Error("Server exited before expected diagnostic");
      await new Promise((resolve) => setTimeout(resolve, 50));
    }
    throw new Error(
      `Expected LSP notification timed out; received ${this.messages.map((m) => m.method).join(", ")}`,
    );
  }
  async initialize(folder, initializationOptions = {}) {
    await this.request("initialize", {
      processId: process.pid,
      rootUri: uri(folder),
      workspaceFolders: [{ uri: uri(folder), name: "moto" }],
      capabilities: {
        workspace: { configuration: true },
        textDocument: { publishDiagnostics: {} },
      },
      initializationOptions,
    });
    this.notify("initialized", {});
  }
  open(path, languageId, text) {
    this.notify("textDocument/didOpen", {
      textDocument: { uri: uri(path), languageId, version: 1, text },
    });
  }
  async close() {
    if (this.process.exitCode !== null) return;
    await this.request("shutdown", null);
    this.notify("exit");
  }
}

function fixture(path, text) {
  writeFileSync(path, text, { flag: "wx" });
  temporary.push(path);
  return path;
}
try {
  const go = new Lsp("gopls", []);
  await go.initialize(root);
  const goText =
    'package main\nvar environmentLSPProbe int = "wrong"\nfunc environmentLSPReference() int { return environmentLSPProbe }\n';
  const goPath = fixture(
    join(root, `backend/lsp_probe_${process.pid}.go`),
    goText,
  );
  go.open(goPath, "go", goText);
  await go.notification(
    (m) =>
      m.method === "textDocument/publishDiagnostics" &&
      m.params.uri === uri(goPath) &&
      m.params.diagnostics.some((d) => d.severity === 1),
  );
  const goDefinition = await go.request("textDocument/definition", {
    textDocument: { uri: uri(goPath) },
    position: { line: 2, character: 45 },
  });
  assert.ok(
    JSON.stringify(goDefinition).includes(uri(goPath)),
    "Go definition must resolve",
  );
  console.log("PASS Go: nested module, type-error diagnostic, definition");
  await go.close();
  rmSync(goPath);

  const expectedTs = JSON.parse(
    readFileSync(join(root, "frontend/node_modules/typescript/package.json")),
  ).version;
  const folder = join(root, `frontend/src/app/lsp-probe-${process.pid}`);
  mkdirSync(folder);
  temporary.push(folder);
  const text =
    'import { useState } from "react";\nconst answer: number = "wrong";\nexport default function Page() { return <div>{answer}{String(useState)}</div>; }\n';
  const tsPath = fixture(join(folder, "page.tsx"), text);
  for (const [client, workspace, options] of [
    ["Zed", root, zed.lsp["typescript-language-server"].initialization_options],
    [
      "Claude",
      claude.typescript.workspaceFolder.replace("${CLAUDE_PROJECT_DIR}", root),
      claude.typescript.initializationOptions,
    ],
  ]) {
    const ts = new Lsp("typescript", ["--stdio"]);
    await ts.initialize(workspace, options);
    const version = await ts.notification(
      (m) => m.method === "$/typescriptVersion",
    );
    assert.equal(
      version.params.version,
      expectedTs,
      "Server must use workspace TypeScript",
    );
    ts.open(tsPath, "typescriptreact", text);
    const diagnostics = await ts.notification(
      (m) =>
        m.method === "textDocument/publishDiagnostics" &&
        m.params.uri === uri(tsPath) &&
        m.params.diagnostics.some((d) => d.code === 71001),
    );
    assert.ok(
      diagnostics.params.diagnostics.some((d) => d.code === 2322),
      "TypeScript type error must be reported",
    );
    const definition = await ts.request("textDocument/definition", {
      textDocument: { uri: uri(tsPath) },
      position: { line: 2, character: 46 },
    });
    assert.ok(
      JSON.stringify(definition).includes(uri(tsPath)),
      "TypeScript definition must resolve",
    );
    console.log(
      `PASS ${client} TypeScript ${expectedTs}: definition, type error, Next server-component diagnostic`,
    );
    await ts.close();
  }

  const ox = new Lsp("oxlint", ["--lsp"]);
  await ox.initialize(root);
  const lintText =
    'export const badDate = new Date().toISOString().split("T")[0];\n';
  const lintPath = fixture(
    join(root, `frontend/src/lsp-probe-${process.pid}.ts`),
    lintText,
  );
  ox.open(lintPath, "typescript", lintText);
  await ox.notification(
    (m) =>
      m.method === "textDocument/publishDiagnostics" &&
      m.params.uri === uri(lintPath) &&
      JSON.stringify(m.params.diagnostics).includes("no-utc-date-extraction"),
  );
  console.log("PASS Oxlint: repository custom date-safety diagnostic");
  await ox.close();

  const prettier = spawnSync(
    join(root, "scripts/editor-tool.sh"),
    ["prettier", "--stdin-filepath", tsPath],
    {
      cwd: root,
      encoding: "utf8",
      input: 'export const sample=<div className="p-4 flex" />',
      timeout,
    },
  );
  assert.equal(prettier.status, 0, prettier.stderr);
  assert.ok(
    prettier.stdout.includes('className="flex p-4"'),
    "Prettier must load the Tailwind plugin",
  );
  console.log("PASS Prettier: repository configuration and Tailwind sorting");
} finally {
  cleanup();
  process.removeListener("exit", cleanup);
}
