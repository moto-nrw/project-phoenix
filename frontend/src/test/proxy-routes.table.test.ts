import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { readProxyRouteExpectations } from "./proxy-routes.table";

const ownedDirectories: string[] = [];
afterEach(() => {
  for (const directory of ownedDirectories.splice(0)) {
    rmSync(directory, { recursive: true, force: true });
  }
});
function fixture(files: Record<string, string>) {
  const directory = mkdtempSync(
    path.join(tmpdir(), "phoenix-proxy-expectations-"),
  );
  ownedDirectories.push(directory);
  for (const [file, contents] of Object.entries(files)) {
    const filename = path.join(directory, file);
    mkdirSync(path.dirname(filename), { recursive: true });
    writeFileSync(filename, contents);
  }
  return directory;
}

describe("reviewed proxy fragments", () => {
  it("discovers new capabilities without changing an import registry", () => {
    expect(
      readProxyRouteExpectations(
        fixture({
          "api/new-capability.json":
            '{"api/new-capability":["GET tenant /api/new-capability"]}',
          "api/school/calendar.json":
            '{"api/school/calendar":["GET school /school/calendar"]}',
        }),
      ),
    ).toEqual({
      "api/new-capability": ["GET tenant /api/new-capability"],
      "api/school/calendar": ["GET school /school/calendar"],
    });
  });
  it.each([
    ["empty", {}],
    ["empty part", { "api/new.json": "{}" }],
    [
      "wrong capability",
      { "api/wrong.json": '{"api/new":["GET tenant /api/new"]}' },
    ],
    [
      "duplicate property",
      {
        "api/new.json":
          '{"api/new":["GET tenant /api/new"],"api/\\u006eew":["POST tenant /api/new"]}',
      },
    ],
    ["malformed", { "api/new.json": "{" }],
    ["bad calls", { "api/new.json": '{"api/new":[]}' }],
    ["unknown input", { "api/new.txt": "ignored?" }],
  ] satisfies Array<[string, Record<string, string>]>)(
    "rejects %s instead of returning a partial green table",
    (_name, files) => {
      expect(() => readProxyRouteExpectations(fixture(files))).toThrow();
    },
  );
});
