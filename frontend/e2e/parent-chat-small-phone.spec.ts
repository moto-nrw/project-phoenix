import { expect, test, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

import { frontendPort } from "./frontend-port";

// Eltern-Chat auf einem kleinen Android-Handy (360 × 640, Touch). Nach dem
// Release vom 04.11.2026 blieben dem Nachrichtenverlauf dort 24 px, die Seite
// ließ sich nicht scrollen und das Schreibfeld war abgeschnitten. Die Regeln
// prüft `use-chat-viewport-lock.test.tsx`; hier geht es um das, was nur die
// echte Seite zeigt: Kopf, Gruppennachrichten, Schnellaktionen und Schreibfeld
// zusammen.

interface SeedParent {
  readonly email?: string;
  readonly password?: string;
  readonly student_ids?: readonly unknown[];
}

function seedParentWithOneChild(): SeedParent | null {
  try {
    const state = JSON.parse(
      readFileSync(
        join(process.cwd(), "..", "backend", ".seed-state.json"),
        "utf8",
      ),
    ) as {
      default_profile?: string;
      profiles?: Record<string, { credentials?: { parents?: SeedParent[] } }>;
    };
    const profile = state.profiles?.[state.default_profile ?? ""];
    return (
      profile?.credentials?.parents?.find(
        (parent) =>
          parent.email && parent.password && parent.student_ids?.length === 1,
      ) ?? null
    );
  } catch {
    return null;
  }
}

const parent = seedParentWithOneChild();
const parentsHost =
  process.env.NEXT_PUBLIC_PARENTS_HOSTNAME ??
  `eltern.localhost:${frontendPort}`;
const base = `http://${parentsHost}`;

if (process.env.CI && parent === null) {
  throw new Error(
    "E2E in CI braucht ein Elternkonto mit einem Kind in backend/.seed-state.json.",
  );
}

test.use({
  viewport: { width: 360, height: 640 },
  isMobile: true,
  hasTouch: true,
  locale: "de-DE",
});

async function openMessages(page: Page) {
  await page.goto(`${base}/messages`, { waitUntil: "domcontentloaded" });
  const email = page.getByLabel("E-Mail-Adresse");
  const composer = page.getByRole("textbox", { name: /Nachricht/ });
  await expect(email.or(composer).first()).toBeVisible({ timeout: 60_000 });
  if (await email.isVisible()) {
    await email.fill(parent!.email!);
    await page.locator("#parent-password").fill(parent!.password!);
    await page.getByRole("button", { name: "Anmelden", exact: true }).click();
    await page.waitForURL((url) => !url.pathname.includes("/login"));
    await page.goto(`${base}/messages`, { waitUntil: "domcontentloaded" });
  }
  await expect(composer).toBeVisible({ timeout: 60_000 });
  return composer;
}

/** Höhen in CSS-Pixeln, gemessen, sobald der Hook die Größe gesetzt hat. */
async function chatLayout(page: Page) {
  return page.evaluate(() => {
    const list = document.querySelector<HTMLElement>("[data-chat-scroll]");
    const chat = list?.parentElement?.closest<HTMLElement>("[style*=height]");
    const main = document.querySelector("main");
    const navTop =
      window.innerHeight -
      (main ? parseFloat(getComputedStyle(main).paddingBottom) || 0 : 0);
    return {
      list: list?.clientHeight ?? 0,
      clipped: chat ? chat.scrollHeight - chat.clientHeight : 0,
      navTop,
    };
  });
}

test.describe("Eltern-Chat auf einem kleinen Handy", () => {
  test.skip(parent === null, "Elternkonto mit einem Kind im Seed erforderlich");

  test("zeigt Verlauf und Schreibfeld vollständig, auch mit Gruppennachrichten", async ({
    page,
  }) => {
    const composer = await openMessages(page);

    await expect
      .poll(async () => (await chatLayout(page)).list)
      .toBeGreaterThanOrEqual(160);
    const layout = await chatLayout(page);
    expect(
      layout.clipped,
      "Chat schneidet seinen Inhalt ab",
    ).toBeLessThanOrEqual(1);

    // Passt der Chat nicht unter die Gruppennachrichten, scrollt die Seite;
    // ganz unten steht das Schreibfeld über der Navigation.
    await page.mouse.wheel(0, 5000);
    await expect
      .poll(async () => (await composer.boundingBox())?.y ?? Infinity)
      .toBeLessThan(layout.navTop);
    const box = await composer.boundingBox();
    expect(box!.y + box!.height).toBeLessThanOrEqual(layout.navTop);
  });

  test("passt ohne Gruppennachrichten auf den Bildschirm", async ({ page }) => {
    await page.route("**/api/parent/me/messages/group-threads", (route) =>
      route.fulfill({ json: { data: [] } }),
    );
    const composer = await openMessages(page);

    await expect
      .poll(async () => (await chatLayout(page)).list)
      .toBeGreaterThanOrEqual(160);
    const layout = await chatLayout(page);
    const box = await composer.boundingBox();
    expect(box!.y + box!.height).toBeLessThanOrEqual(layout.navTop);
  });
});
