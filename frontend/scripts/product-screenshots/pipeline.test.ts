import {
  mkdtemp,
  mkdir,
  readFile,
  readdir,
  rm,
  writeFile,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, expect, test, vi } from "vitest";
import sharp from "sharp";

import { loginToLocalTenant } from "../../e2e/local-tenant-login";
import type { StackAccess } from "./access";
import * as captures from "./capture";
import { sessionFor } from "./capture";
import * as frames from "./frame";
import { captureInOrder } from "./capture-order";
import { mapConcurrent } from "./concurrency";
import { BrokenShotError } from "./quality";
import { parseShotList, type Shot } from "./shot-list";

vi.mock("../../e2e/local-tenant-login", () => ({
  loginToLocalTenant: vi.fn(),
  localMFACode: vi.fn(),
}));

import {
  assertReplaceable,
  readOutput,
  replaceOutput,
  runPipeline,
} from "./pipeline";

const directories: string[] = [];

afterEach(async () => {
  vi.restoreAllMocks();
  await Promise.all(
    directories
      .splice(0)
      .map((dir) => rm(dir, { recursive: true, force: true })),
  );
});

test("ein fehlgeschlagener Austausch erhält die vorige Ausgabe", async () => {
  const root = await mkdtemp(join(tmpdir(), "product-screenshots-"));
  directories.push(root);
  const outDir = join(root, "ausgabe");
  await mkdir(outDir);
  await writeFile(join(outDir, "manifest.json"), "bisherige Ausgabe");

  await expect(
    replaceOutput(join(root, "fehlende-vorbereitung"), outDir),
  ).rejects.toThrow();

  expect(await readFile(join(outDir, "manifest.json"), "utf8")).toBe(
    "bisherige Ausgabe",
  );
  expect(await readdir(root)).toEqual(["ausgabe"]);
});

test("ein fremdes Manifest erlaubt keinen Austausch", async () => {
  const root = await mkdtemp(join(tmpdir(), "product-screenshots-"));
  directories.push(root);
  const outDir = join(root, "ausgabe");
  await mkdir(outDir);
  await writeFile(
    join(outDir, "manifest.json"),
    JSON.stringify({ version: "fremd" }),
  );
  await writeFile(join(outDir, "notizen.txt"), "behalten");

  await expect(assertReplaceable(outDir)).rejects.toThrow(/Pipeline/);
  expect(await readFile(join(outDir, "notizen.txt"), "utf8")).toBe("behalten");
});

test("nicht gelistete Dateien in einer Pipeline-Ausgabe bleiben geschützt", async () => {
  const root = await mkdtemp(join(tmpdir(), "product-screenshots-"));
  directories.push(root);
  const outDir = join(root, "ausgabe");
  await mkdir(outDir);
  await writeFile(
    join(outDir, "manifest.json"),
    JSON.stringify({ format: "moto-product-screenshots/v1", shots: [] }),
  );
  await writeFile(join(outDir, "notizen.txt"), "behalten");

  await expect(assertReplaceable(outDir)).rejects.toThrow(/notizen\.txt/);
  expect(await readFile(join(outDir, "notizen.txt"), "utf8")).toBe("behalten");
});

test("akzeptiert nur vollständig gelistete Pipeline-Dateien", async () => {
  const root = await mkdtemp(join(tmpdir(), "product-screenshots-"));
  directories.push(root);
  const outDir = join(root, "ausgabe");
  await mkdir(join(outDir, "shot"), { recursive: true });
  await writeFile(join(outDir, "shot", "macbook.png"), "bild");
  await writeFile(
    join(outDir, "manifest.json"),
    JSON.stringify({
      format: "moto-product-screenshots/v1",
      shots: [{ id: "shot", dateien: [{ pfad: "shot/macbook.png" }] }],
    }),
  );

  await expect(assertReplaceable(outDir)).resolves.toBeUndefined();
});

function deferred() {
  let resolve = () => {};
  const promise = new Promise<void>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

test("bounded work overlaps but keeps the catalog order", async () => {
  const gates = [deferred(), deferred(), deferred()];
  const firstPair = deferred();
  const thirdStarted = deferred();
  const started: number[] = [];
  const pending = mapConcurrent([0, 1, 2], 2, async (index) => {
    started.push(index);
    if (started.length === 2) firstPair.resolve();
    if (index === 2) thirdStarted.resolve();
    await gates[index]!.promise;
    return index;
  });
  await firstPair.promise;
  expect(started).toEqual([0, 1]);
  gates[1]!.resolve();
  await thirdStarted.promise;
  expect(started).toEqual([0, 1, 2]);
  gates[2]!.resolve();
  gates[0]!.resolve();
  expect(await pending).toEqual([0, 1, 2]);
});

test("a failed task drains active work before cleanup and starts no queued work", async () => {
  const active = deferred();
  const failing = deferred();
  const firstPair = deferred();
  const started: number[] = [];
  let settled = false;
  const error = new Error("broken image");
  const pending = mapConcurrent([0, 1, 2], 2, async (index) => {
    started.push(index);
    if (started.length === 2) firstPair.resolve();
    if (index === 0) {
      await active.promise;
      return index;
    }
    await failing.promise;
    throw error;
  });
  const rejected = expect(
    pending.finally(() => {
      settled = true;
    }),
  ).rejects.toBe(error);
  await firstPair.promise;
  failing.resolve();
  await Promise.resolve();
  expect(started).toEqual([0, 1]);
  expect(settled).toBe(false);
  active.resolve();
  await rejected;
  expect(started).toEqual([0, 1]);
});

function catalog(...paths: string[]): Shot[] {
  return parseShotList(`shots:
${paths
  .map(
    (path, index) => `  - id: shot-${index}
    titel: Shot ${index}
    portal: eltern
    rolle: eltern
    pfad: ${path}
    geraete: [iphone]`,
  )
  .join("\n")}`);
}

test.each(["/news", "/admin/enrollments"])(
  "capture preserves the %s shared-read barrier around parallel shots",
  async (barrier) => {
    const shots = catalog("/children", "/calendar", barrier, "/meal-plan");
    const before = deferred();
    const pairStarted = deferred();
    const communication = deferred();
    const communicationStarted = deferred();
    const events: string[] = [];
    const pending = captureInOrder(
      shots,
      2,
      async (shot) => {
        events.push(shot.id);
        if (shot.id === "shot-0") await before.promise;
        if (shot.id === "shot-1") pairStarted.resolve();
        if (shot.id === "shot-2") {
          communicationStarted.resolve();
          await communication.promise;
        }
        return shot.id;
      },
      () => {},
    );
    await pairStarted.promise;
    expect(events).toEqual(["shot-0", "shot-1"]);
    before.resolve();
    await communicationStarted.promise;
    expect(events).toEqual(["shot-0", "shot-1", "shot-2"]);
    communication.resolve();
    expect(await pending).toEqual(shots.map((shot) => shot.id));
    expect(events).toEqual(shots.map((shot) => shot.id));
  },
);

test("capture preserves broken-shot validation and skips its remaining devices", async () => {
  const [shot] = catalog("/children");
  const shots: Shot[] = [{ ...shot!, geraete: ["iphone", "ipad"] }];
  const captured: string[] = [];
  const broken = new BrokenShotError(shot!.id, "iphone", ["loading indicator"]);
  const reported: BrokenShotError[] = [];
  await expect(
    captureInOrder(
      shots,
      4,
      async (_shot, device) => {
        captured.push(device);
        throw broken;
      },
      (error) => reported.push(error),
    ),
  ).rejects.toThrow(/loading indicator/);
  expect(captured).toEqual(["iphone"]);
  expect(reported).toEqual([broken]);
});

test("concurrent shots share one in-flight login instead of racing MFA", async () => {
  const loginDone = deferred();
  const loginStarted = deferred();
  vi.mocked(loginToLocalTenant).mockImplementation(async () => {
    loginStarted.resolve();
    await loginDone.promise;
  });
  const state = { cookies: [], origins: [] };
  const page = { close: vi.fn() };
  const context = {
    newPage: vi.fn().mockResolvedValue(page),
    storageState: vi.fn().mockResolvedValue(state),
    close: vi.fn(),
  };
  const browser = { newContext: vi.fn().mockResolvedValue(context) };
  const access: StackAccess = {
    tenantOrigin: "http://school.localhost:3000",
    parentsOrigin: "http://eltern.localhost:3000",
    traegerOrigin: null,
    admin: { email: "admin@example.org", password: "fixture" },
    betreuer: null,
    eltern: null,
    traeger: null,
  };
  const shots = parseShotList(`shots:
  - id: home
    titel: Home
    portal: tenant
    rolle: admin
    pfad: /home
`);
  const cache = new Map();
  const first = sessionFor(browser, cache, access, shots[0]!);
  const second = sessionFor(browser, cache, access, shots[0]!);
  await loginStarted.promise;
  expect(browser.newContext).toHaveBeenCalledTimes(1);
  expect(loginToLocalTenant).toHaveBeenCalledTimes(1);
  loginDone.resolve();
  expect(await Promise.all([first, second])).toEqual([state, state]);
  expect(context.storageState).toHaveBeenCalledTimes(1);
  expect(context.close).toHaveBeenCalledTimes(1);
});

const fixtureAccess: StackAccess = {
  tenantOrigin: "http://school.localhost:3000",
  parentsOrigin: "http://eltern.localhost:3000",
  traegerOrigin: null,
  admin: { email: "admin@example.org", password: "fixture" },
  betreuer: null,
  eltern: null,
  traeger: null,
};

async function prepareFrameRun() {
  const root = await mkdtemp(join(tmpdir(), "product-screenshots-"));
  directories.push(root);
  const shots = catalog("/children", "/calendar", "/meal-plan");
  const png = await sharp({
    create: {
      width: 2,
      height: 2,
      channels: 4,
      background: "#123456",
    },
  })
    .png()
    .toBuffer();
  vi.spyOn(captures, "captureShots").mockResolvedValue(
    shots.map((shot) => ({
      shotId: shot.id,
      device: "iphone",
      png,
    })),
  );
  vi.spyOn(frames, "webpVariants").mockResolvedValue(
    frames.WEBP_WIDTHS.map((width) => ({
      width,
      height: width,
      data: png,
      upscaled: true,
    })),
  );
  return { shots, png, outDir: join(root, "output") };
}

test("parallel framing keeps every PNG and all six variants in stable manifest order", async () => {
  const { shots, png, outDir } = await prepareFrameRun();
  const gates = [deferred(), deferred(), deferred()];
  const firstPair = deferred();
  const third = deferred();
  let calls = 0;
  vi.spyOn(frames, "frameMockup").mockImplementation(async () => {
    const index = calls++;
    if (calls === 2) firstPair.resolve();
    if (calls === 3) third.resolve();
    await gates[index]!.promise;
    return png;
  });
  const pending = runPipeline({
    access: fixtureAccess,
    shots,
    outDir,
    version: "parallel",
    frameConcurrency: 2,
  });
  await firstPair.promise;
  expect(calls).toBe(2);
  gates[1]!.resolve();
  await third.promise;
  expect(calls).toBe(3);
  gates[2]!.resolve();
  gates[0]!.resolve();
  const manifest = await pending;
  expect(await readOutput(outDir)).toEqual(manifest);
  expect(manifest.shots.map((shot) => shot.id)).toEqual(
    shots.map((shot) => shot.id),
  );
  for (const shot of manifest.shots) {
    expect(shot.dateien.map((file) => file.pfad)).toEqual([
      `${shot.id}/roh-iphone.png`,
      ...frames.WEBP_WIDTHS.map(
        (width) => `${shot.id}/roh-iphone-${width}.webp`,
      ),
      `${shot.id}/iphone.png`,
      ...frames.WEBP_WIDTHS.map((width) => `${shot.id}/iphone-${width}.webp`),
    ]);
  }
});

test("a parallel framing failure leaves the previous complete output untouched", async () => {
  const { shots, png, outDir } = await prepareFrameRun();
  await mkdir(outDir);
  const previous = JSON.stringify({
    format: "moto-product-screenshots/v1",
    shots: [],
  });
  await writeFile(join(outDir, "manifest.json"), previous);
  const busy = deferred();
  const firstPair = deferred();
  let calls = 0;
  vi.spyOn(frames, "frameMockup").mockImplementation(async () => {
    const index = calls++;
    if (calls === 2) firstPair.resolve();
    if (index === 0) {
      await busy.promise;
      return png;
    }
    throw new Error("invalid bezel");
  });
  const pending = runPipeline({
    access: fixtureAccess,
    shots,
    outDir,
    version: "broken",
    frameConcurrency: 2,
  });
  const rejected = expect(pending).rejects.toThrow("invalid bezel");
  await firstPair.promise;
  expect(await readFile(join(outDir, "manifest.json"), "utf8")).toBe(previous);
  busy.resolve();
  await rejected;
  expect(calls).toBe(2);
  expect(await readFile(join(outDir, "manifest.json"), "utf8")).toBe(previous);
});

test.each([0, -1, 1.5, Number.NaN])(
  "rejects invalid concurrency %s before starting work",
  async (concurrency) => {
    const run = vi.fn().mockResolvedValue("image");
    await expect(mapConcurrent(["shot"], concurrency, run)).rejects.toThrow(
      /positive integer/,
    );
    expect(run.mock.calls).toEqual([]);
  },
);
