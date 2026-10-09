import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";

// Node answers 431 to a request whose headers exceed its limit, before
// Next.js sees it (#3883). A tenant session alone needs about 8 KB (session
// cookie chunks plus the Authorization header), so the default of 16 KB left
// no room for cookies of other moto hosts on the parent domain.
const MIN_HEADER_BYTES = 64 * 1024;

const DOCKERFILES = ["Dockerfile", "Dockerfile.prod"];
const NATIVE_DEV_SCRIPT = path.resolve(
  process.cwd(),
  "..",
  "scripts",
  "dev-native.sh",
);
const PACKAGE_JSON = path.resolve(process.cwd(), "package.json");
const NEXT_CLI = "node_modules/next/dist/bin/next";

function nodeCommand(dockerfile: string, entrypoint: string): string[] {
  const source = readFileSync(path.join(process.cwd(), dockerfile), "utf8");
  const commands = [...source.matchAll(/^CMD (\[.*\])$/gm)].map(
    (match) => JSON.parse(match[1] ?? "[]") as string[],
  );
  const matching = commands.filter((command) => command.includes(entrypoint));
  expect(matching, `${dockerfile} starts ${entrypoint} once`).toHaveLength(1);
  return matching[0] ?? [];
}

function headerLimit(command: string[]): number | undefined {
  const flag = command.find((arg) => arg.startsWith("--max-http-header-size="));
  return flag ? Number(flag.split("=")[1]) : undefined;
}

function nativeDevCommand(): string[] {
  const source = readFileSync(NATIVE_DEV_SCRIPT, "utf8");
  const match = source.match(
    /^  PORT=\$FRONTEND_HOST_PORT start_svc frontend frontend (.+)$/m,
  );
  const command = match?.[1];
  expect(command, "native development starts the frontend once").toBeDefined();
  return command?.split(" ") ?? [];
}

function packageScript(name: "dev" | "start" | "preview"): string {
  const packageJson = JSON.parse(readFileSync(PACKAGE_JSON, "utf8")) as {
    scripts: Record<string, string | undefined>;
  };
  const script = packageJson.scripts[name];
  expect(script, `package script ${name} exists`).toBeDefined();
  return script ?? "";
}

describe("frontend server header limit", () => {
  it("raises the Node header limit for the development server", () => {
    const command = nodeCommand(
      "Dockerfile",
      "node_modules/next/dist/bin/next",
    );
    expect(command[0]).toBe("node");
    expect(command.slice(-1)).toEqual(["dev"]);
    expect(headerLimit(command)).toBeGreaterThanOrEqual(MIN_HEADER_BYTES);
  });

  it("raises the Node header limit for the native development server", () => {
    const command = nativeDevCommand();
    expect(command[0]).toBe("node");
    expect(command.slice(-1)).toEqual(["dev"]);
    expect(headerLimit(command)).toBeGreaterThanOrEqual(MIN_HEADER_BYTES);
  });

  it.each([
    ["dev", "dev"],
    ["start", "start"],
  ] as const)("raises the Node header limit for pnpm run %s", (name, mode) => {
    const command = packageScript(name);
    expect(command).toBe(
      `node --max-http-header-size=65536 ${NEXT_CLI} ${mode}`,
    );
  });

  it("raises the Node header limit for pnpm run preview", () => {
    expect(packageScript("preview")).toBe(
      `next build && node --max-http-header-size=65536 ${NEXT_CLI} start`,
    );
  });

  it.each(DOCKERFILES)(
    "%s raises the Node header limit for server.js",
    (dockerfile) => {
      const command = nodeCommand(dockerfile, "server.js");
      expect(command[0]).toBe("node");
      expect(command.indexOf("server.js")).toBe(command.length - 1);
      expect(headerLimit(command)).toBeGreaterThanOrEqual(MIN_HEADER_BYTES);
    },
  );

  it("uses the same limit for development and deployed servers", () => {
    const limits = [
      headerLimit(nodeCommand("Dockerfile", "node_modules/next/dist/bin/next")),
      headerLimit(nativeDevCommand()),
      headerLimit(packageScript("dev").split(" ")),
      headerLimit(packageScript("start").split(" ")),
      headerLimit(packageScript("preview").split(" ")),
      ...DOCKERFILES.map((file) => headerLimit(nodeCommand(file, "server.js"))),
    ];
    expect(new Set(limits).size).toBe(1);
  });
});
