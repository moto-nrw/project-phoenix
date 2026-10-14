import type { Page, Request } from "@playwright/test";

// Wann ein Warten der Produkt-Screenshots aufgibt (#3759). Der Dev-Server
// kompiliert jede Route beim ersten Aufruf und antwortet auf kleinen
// CI-Runnern langsam. Die Kinderseite der Eltern lädt rund 15 Routen; der
// Browser schickt höchstens sechs Anfragen zugleich an einen Host, und eine
// davon hält der SSE-Strom. So braucht die Seite im CI 20 bis 40 Sekunden,
// ohne kaputt zu sein. Ein festes Zeitlimit kann "lädt noch" und "hängt"
// nicht unterscheiden. Deshalb wartet die Pipeline, solange die Seite Anfragen
// an den eigenen Stack offen hat, und gibt auf, wenn sie eine Weile nichts
// mehr lädt oder die Obergrenze erreicht ist.

/** Obergrenze für ein einzelnes Warten, auch wenn die Seite noch lädt. */
export const MAX_WAIT_MS = 120_000;

export interface NetworkState {
  /** Offene Anfragen der Seite an den eigenen Stack, ohne SSE-Ströme. */
  readonly inFlight: number;
  /** Zeitpunkt (ms), zu dem zuletzt eine Anfrage begann oder endete. */
  readonly lastActivityAt: number;
}

export interface WaitLimits {
  /** So lange darf die Seite nichts laden, bevor das Warten aufgibt. */
  readonly idleMs: number;
  /** So lange wartet es höchstens, auch während die Seite noch lädt. */
  readonly maxMs: number;
}

/**
 * Ob ein Warten, das zu `startedAt` begann, aufgeben muss. Eine Seite, die
 * nichts lädt, gibt nach `idleMs` auf, genau wie mit einem festen Limit. Nur
 * solange sie Antworten des Stacks erwartet, wartet die Pipeline länger.
 */
export function waitExpired(
  now: number,
  startedAt: number,
  network: NetworkState,
  limits: WaitLimits,
): boolean {
  if (now - startedAt >= limits.maxMs) return true;
  if (network.inFlight > 0) return false;
  const idleSince = Math.max(network.lastActivityAt, startedAt);
  return now - idleSince >= limits.idleMs;
}

export interface NetworkActivity {
  state(): NetworkState;
}

/**
 * Zählt ab jetzt die offenen Anfragen der Seite an Hosts, für die `ownHost`
 * gilt. EventSource-Verbindungen bleiben offen, solange die Seite lebt; sie
 * zählen nicht.
 */
export function trackNetworkActivity(
  page: Page,
  ownHost: (url: string) => boolean,
): NetworkActivity {
  const open = new Set<Request>();
  let lastActivityAt = Date.now();
  const counts = (request: Request) =>
    request.resourceType() !== "eventsource" && ownHost(request.url());
  page.on("request", (request) => {
    if (!counts(request)) return;
    open.add(request);
    lastActivityAt = Date.now();
  });
  const settle = (request: Request) => {
    if (!open.delete(request)) return;
    lastActivityAt = Date.now();
  };
  page.on("requestfinished", settle);
  page.on("requestfailed", settle);
  return { state: () => ({ inFlight: open.size, lastActivityAt }) };
}
