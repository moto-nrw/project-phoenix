import { expect, test, type Page } from "@playwright/test";
import {
  box,
  compileGlobalsCss,
  render as renderWithCss,
  tenantPage,
} from "./harness";

// Die Wachs-Regel des Seitenrumpfs (`.moto-tenant-body` in globals.css) im
// echten Browser: Die letzte Fläche einer Tenant-Seite wächst bis zur
// Unterkante, eine `moto-scroll-surface` (die Chats) nicht (#3328).
//
// Das Markup bildet `TenantPage` und die Chat-Seiten nach
// (`messages/[threadId]`, `team-chat/[threadID]`); das CSS ist das echte
// globals.css, durch Tailwind kompiliert. Die Klassen stehen hier im Klartext,
// damit Tailwinds Quellensuche sie findet.

// SectionCard ohne Titel (section-card.tsx) plus die Klassen der Chat-Seite.
const CHAT_CARD =
  "moto-content-surface overflow-hidden compact:p-4 rounded-2xl border p-5 shadow-sm backdrop-blur-md max-sm:p-4 flex min-h-0 flex-1 flex-col";
const PLAIN_CARD =
  "moto-content-surface overflow-hidden compact:p-4 rounded-2xl border p-5 shadow-sm backdrop-blur-md max-sm:p-4";

// Höhe, die `useChatViewportLock` der Hülle als Inline-Stil gibt.
const LOCKED_HEIGHT = 400;
const MESSAGE_COUNT = 15;
const MESSAGE_HEIGHT = 80;

let compiledCss = "";

test.beforeAll(async () => {
  compiledCss = await compileGlobalsCss();
});

function chatPage(cardClass: string): string {
  const messages = Array.from(
    { length: MESSAGE_COUNT },
    (_, i) =>
      `<div style="height:${MESSAGE_HEIGHT}px">Nachricht ${i + 1}</div>`,
  ).join("");
  return tenantPage(`
    <a href="#">Zurück zu Nachrichten</a>
    <div data-testid="container" class="flex min-h-[20rem] w-full flex-col overflow-hidden" style="height:${LOCKED_HEIGHT}px">
      <section data-testid="card" class="${cardClass}">
        <div class="flex min-h-0 flex-1 flex-col">
          <div data-testid="list" class="min-h-0 flex-1 space-y-3 overflow-y-auto pr-1">
            ${messages}
            <section class="${PLAIN_CARD}">Frühere Anfrage</section>
          </div>
          <div class="mt-4">
            <textarea data-testid="composer" aria-label="Nachricht"></textarea>
          </div>
        </div>
      </section>
    </div>`);
}

async function render(page: Page, body: string): Promise<void> {
  await renderWithCss(page, compiledCss, body);
}

test("a scroll surface keeps its locked height, scrolls and shows the composer", async ({
  page,
}) => {
  await render(page, chatPage(`moto-scroll-surface ${CHAT_CARD}`));

  const container = await box(page, "container");
  const card = await box(page, "card");
  expect(card.height).toBeLessThanOrEqual(container.height);

  const list = page.getByTestId("list");
  const overflow = await list.evaluate(
    (el) => el.scrollHeight - el.clientHeight,
  );
  expect(overflow).toBeGreaterThan(0);

  const composer = await box(page, "composer");
  expect(composer.y + composer.height).toBeLessThanOrEqual(
    container.y + container.height,
  );
  await expect(page.getByTestId("composer")).toBeVisible();
  await expect(page.getByTestId("composer")).toBeInViewport();
});

test("without the exit the grow rule bloats the chat card and clips the composer", async ({
  page,
}) => {
  // Belegt, dass der Nachbau den Fehler aus #3328 trifft: fehlt die Klasse,
  // wird die Karte so hoch wie der ganze Verlauf.
  await render(page, chatPage(CHAT_CARD));

  const container = await box(page, "container");
  const card = await box(page, "card");
  expect(card.height).toBeGreaterThan(container.height);

  const composer = await box(page, "composer");
  expect(composer.y).toBeGreaterThan(container.y + container.height);
});

test("the last surface of an ordinary tenant page still grows to the bottom", async ({
  page,
}) => {
  await render(
    page,
    tenantPage(`
      <section class="${PLAIN_CARD}">Kennzahlen</section>
      <div class="space-y-6">
        <section class="${PLAIN_CARD}">Filter</section>
        <section data-testid="last" class="${PLAIN_CARD}">Ein Eintrag</section>
      </div>`),
  );

  const shell = await box(page, "shell");
  const last = await box(page, "last");
  expect(Math.round(last.y + last.height)).toBe(
    Math.round(shell.y + shell.height),
  );
});
