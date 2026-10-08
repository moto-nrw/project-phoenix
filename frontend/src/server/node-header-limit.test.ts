import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";

// Node answers 431 to a request whose headers exceed its limit, before
// Next.js sees it (#3883). A tenant session alone needs about 8 KB (session
// cookie chunks plus the Authorization header), so the default of 16 KB left
// no room for cookies of other moto hosts on the parent domain.
const MIN_HEADER_BYTES = 64 * 1024;

const DOCKERFILES = ["Dockerfile", "Dockerfile.prod"];

function serverCommand(dockerfile: string): string[] {
  const source = readFileSync(path.join(process.cwd(), dockerfile), "utf8");
  const commands = [...source.matchAll(/^CMD (\[.*\])$/gm)].map(
    (match) => JSON.parse(match[1] ?? "[]") as string[],
  );
  const server = commands.filter((command) => command.includes("server.js"));
  expect(server, `${dockerfile} starts server.js once`).toHaveLength(1);
  return server[0] ?? [];
}

function headerLimit(command: string[]): number | undefined {
  const flag = command.find((arg) => arg.startsWith("--max-http-header-size="));
  return flag ? Number(flag.split("=")[1]) : undefined;
}

describe("frontend server header limit", () => {
  it.each(DOCKERFILES)("%s raises the Node header limit", (dockerfile) => {
    const command = serverCommand(dockerfile);
    expect(command[0]).toBe("node");
    expect(command.indexOf("server.js")).toBe(command.length - 1);
    expect(headerLimit(command)).toBeGreaterThanOrEqual(MIN_HEADER_BYTES);
  });

  it("uses the same limit in both images", () => {
    const limits = DOCKERFILES.map((file) => headerLimit(serverCommand(file)));
    expect(new Set(limits).size).toBe(1);
  });
});
