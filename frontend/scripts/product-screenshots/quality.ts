// Qualitätsprüfung der Produkt-Screenshots (#3759). Ein Shot gilt als kaputt
// bei HTTP-Fehler, Fehler-Boundary, Umleitung auf den Login, sichtbarem
// Ladezustand nach Timeout oder Konsolenfehler. Ein kaputter Shot bricht den
// ganzen Lauf ab, damit "Aktuell" nie ein kaputtes Bild enthält. Einen
// visuellen Diff gibt es bewusst nicht; Menschen prüfen die Bilder.
//
// Diese Datei enthält nur die Bewertung. Die Beobachtungen sammelt capture.ts
// im Browser.

export interface FailedResponse {
  readonly method: string;
  readonly url: string;
  readonly status: number;
}

export interface ShotObservation {
  /** Pfad, den der Shot anfordert (`pfad` der Shot-Liste, ohne Query). */
  readonly requestedPath: string;
  /** Pfad, auf dem die Seite nach dem Laden steht. */
  readonly finalPath: string;
  /** Statuscode des Dokuments der Zielseite (null, wenn keine Antwort kam). */
  readonly documentStatus: number | null;
  /** Antworten mit Status >= 400 an den eigenen Stack. */
  readonly failedResponses: readonly FailedResponse[];
  readonly consoleErrors: readonly string[];
  readonly pageErrors: readonly string[];
  /** Sichtbarer Text der Seite. */
  readonly bodyText: string;
  readonly loginFormVisible: boolean;
  readonly devErrorOverlayPresent: boolean;
  /** Ladeanzeigen, die nach dem Warten noch sichtbar sind. */
  readonly loadingIndicators: number;
  /**
   * Sichtbare Dialoge in einem Shot, dessen Vorbereitung keinen Klick enthält.
   * Ein Dialog ist nur gewollt, wenn der Shot ihn selbst öffnet.
   */
  readonly unexpectedDialogs: number;
}

/** Sichtbare Texte der Fehlerseiten des Produkts. */
const ERROR_PAGE_TEXTS = [
  // app/global-error.tsx
  "Ein unerwarteter Fehler ist aufgetreten",
  // app/not-found.tsx: die Route des Shots gibt es nicht
  "Seite nicht gefunden",
  // Next.js, wenn ein Client-Fehler die Seite abräumt
  "Application error: a client-side exception has occurred",
];

/**
 * Antworten, die kein Fehler des Produkts sind. Jeder Eintrag braucht einen
 * Grund; sonst würde ein echter Fehler still durchgehen. Der Browser meldet zu
 * jeder solchen Antwort auch einen Konsolenfehler ("Failed to load resource"),
 * den capture.ts über die URL ebenfalls verwirft.
 */
export const IGNORED_RESPONSES: readonly {
  readonly method: string;
  readonly path: RegExp;
  readonly status: number;
  readonly reason: string;
}[] = [
  {
    method: "GET",
    path: /^\/api\/notifications\/push\/public-key$/,
    status: 404,
    reason:
      "Web-Push ist im lokalen Stack und im CI-Runner nicht konfiguriert (kein VAPID-Schlüssel); die App fragt den Schlüssel trotzdem bei jedem Seitenaufruf ab",
  },
  {
    method: "GET",
    path: /^\/api\/parent\/me\/push\/public-key$/,
    status: 404,
    reason: "Wie oben, für das Eltern-Portal",
  },
];

export function isIgnoredResponse(response: FailedResponse): boolean {
  const { pathname } = new URL(response.url);
  return IGNORED_RESPONSES.some(
    (ignored) =>
      ignored.method === response.method &&
      ignored.status === response.status &&
      ignored.path.test(pathname),
  );
}

function trimSlash(path: string): string {
  return path.length > 1 ? path.replace(/\/+$/, "") : path;
}

/** Gründe, warum der Shot kaputt ist. Leer heißt: in Ordnung. */
export function findBrokenReasons(observation: ShotObservation): string[] {
  const reasons: string[] = [];

  if (observation.documentStatus === null) {
    reasons.push("Die Zielseite hat nicht geantwortet");
  } else if (observation.documentStatus >= 400) {
    reasons.push(
      `Die Zielseite antwortet mit HTTP ${observation.documentStatus}`,
    );
  }
  for (const response of observation.failedResponses) {
    reasons.push(
      `HTTP ${response.status} bei ${response.method} ${response.url}`,
    );
  }

  for (const text of ERROR_PAGE_TEXTS) {
    if (observation.bodyText.includes(text)) {
      reasons.push(`Fehlerseite sichtbar: "${text}"`);
    }
  }
  if (observation.devErrorOverlayPresent) {
    reasons.push("Das Fehler-Overlay des Dev-Servers ist da");
  }

  // Die Tenant-App leitet unbekannte Routen per Client-Redirect auf `/` um,
  // ohne 404 und ohne Fehlertext (app/[tenant]/[...not-found]). Ein Shot muss
  // deshalb dort ankommen, wo er hin wollte.
  if (
    trimSlash(observation.finalPath) !== trimSlash(observation.requestedPath)
  ) {
    reasons.push(
      `Umleitung: angefordert ${observation.requestedPath}, angekommen ${observation.finalPath} (die Route fehlt oder verlangt eine andere Sitzung)`,
    );
  }
  if (observation.loginFormVisible) {
    reasons.push("Umleitung auf den Login: das Anmeldeformular ist sichtbar");
  }
  if (observation.loadingIndicators > 0) {
    reasons.push(
      `${observation.loadingIndicators} Ladeanzeige(n) nach dem Timeout noch sichtbar`,
    );
  }

  if (observation.unexpectedDialogs > 0) {
    reasons.push(
      `${observation.unexpectedDialogs} unerwarteter Dialog im Bild (etwa eine Einrichtungsaufforderung)`,
    );
  }

  for (const message of observation.consoleErrors) {
    reasons.push(`Konsolenfehler: ${message}`);
  }
  for (const message of observation.pageErrors) {
    reasons.push(`Unbehandelter Fehler auf der Seite: ${message}`);
  }
  return reasons;
}

export class BrokenShotError extends Error {
  readonly shotId: string;
  readonly device: string;
  readonly reasons: readonly string[];

  constructor(shotId: string, device: string, reasons: readonly string[]) {
    super(
      `Shot ${shotId} (${device}) ist kaputt, der Lauf bricht ab:\n${reasons
        .map((reason) => `  - ${reason}`)
        .join("\n")}`,
    );
    this.name = "BrokenShotError";
    this.shotId = shotId;
    this.device = device;
    this.reasons = reasons;
  }
}
