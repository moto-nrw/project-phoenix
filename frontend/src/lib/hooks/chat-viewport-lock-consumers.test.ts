import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";

describe("useChatViewportLock — consumers", () => {
  it("is only used by chats that mark their message list", () => {
    // Without `data-chat-scroll` the hook cannot keep the list readable, and
    // a small phone shows the 24 px list from the parents' chat again.
    const srcDir = path.resolve(__dirname, "../..");
    const consumers: string[] = [];
    const walk = (dir: string) => {
      for (const entry of readdirSync(dir, { withFileTypes: true })) {
        const full = path.join(dir, entry.name);
        if (entry.isDirectory()) walk(full);
        else if (
          /\.tsx$/.test(entry.name) &&
          !/\.test\.tsx$/.test(entry.name) &&
          readFileSync(full, "utf8").includes("useChatViewportLock<")
        )
          consumers.push(full);
      }
    };
    walk(srcDir);

    expect(consumers.length).toBeGreaterThanOrEqual(3);
    const unmarked = consumers.filter(
      (file) => !readFileSync(file, "utf8").includes('data-chat-scroll=""'),
    );
    expect(unmarked).toEqual([]);
  });
});
