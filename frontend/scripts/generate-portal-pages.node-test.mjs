import assert from "node:assert/strict";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import test from "node:test";
import {
  PORTALS,
  portalPages,
  synchronizePortalPages,
} from "./generate-portal-pages.mjs";

async function fixture(files, run) {
  const directory = mkdtempSync(join(tmpdir(), "moto-portal-pages-"));
  const app = join(directory, "app");
  for (const portal of PORTALS) mkdirSync(join(app, portal), { recursive: true });
  for (const file of files) {
    mkdirSync(dirname(join(app, file)), { recursive: true });
    writeFileSync(join(app, file), "export default null;\n");
  }
  try {
    await run(app, join(directory, "proxy-pages.generated.ts"));
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
}

test("lists the first segment of every page as the App Router routes it", () =>
  fixture(
    [
      "school/page.tsx",
      "school/login/page.tsx",
      "school/klasse/[id]/page.tsx",
      "school/(signed-in)/kalender/page.tsx",
      "school/_parts/form/page.tsx",
      "school/components/row.tsx",
    ],
    async (app) => {
      assert.deepEqual(portalPages(app, "school"), [
        "/kalender",
        "/klasse",
        "/login",
      ]);
    },
  ));

test("refuses a dynamic first segment, which would make every path a page", () =>
  fixture(["traeger/[slug]/page.tsx"], async (app) => {
    assert.throws(() => portalPages(app, "traeger"), /dynamic first segment/);
  }));

test("check fails once a new page folder is not in the generated list", () =>
  fixture(["caterer/login/page.tsx"], async (app, target) => {
    await synchronizePortalPages({ appDirectory: app, target });
    await synchronizePortalPages({ appDirectory: app, target, check: true });
    mkdirSync(join(app, "caterer", "konto"));
    writeFileSync(join(app, "caterer", "konto", "page.tsx"), "");
    await assert.rejects(
      () => synchronizePortalPages({ appDirectory: app, target, check: true }),
      /stale/,
    );
  }));
