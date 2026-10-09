import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";

// Node answers 431 to a request whose headers exceed its limit, before
// Next.js sees it (#3883). A tenant session alone needs about 8 KB (session
// cookie chunks plus the Authorization header), so the default of 16 KB left
// no room for cookies of other moto hosts on the parent domain.
const MIN_HEADER_BYTES = 64 * 1024;

const DOCKERFILES = ["Dockerfile", "Dockerfile.prod"];

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
      ...DOCKERFILES.map((file) => headerLimit(nodeCommand(file, "server.js"))),
    ];
    expect(new Set(limits).size).toBe(1);
  });
});
