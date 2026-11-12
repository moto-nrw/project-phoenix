/**
 * Welches Navigationsziel zur aktuellen Adresse gehört.
 *
 * Auf dem Träger-Host schreibt der Proxy "/" intern auf /traeger um, die
 * Adresszeile zeigt also "/ferien" statt "/traeger/ferien". Beide
 * Schreibweisen markieren dasselbe Ziel. Gleiches Problem und gleiche Lösung
 * wie `school/shell/school-nav-active.ts`.
 */
const TRAEGER_SETTINGS_HOME = "/traeger/einstellungen";
const TRAEGER_SETTINGS_HOME_ON_HOST = "/einstellungen";

/** Ob die Adresse im Einstellungsbereich liegt, mit oder ohne /traeger (#5685). */
export function isTraegerSettingsPath(pathname: string): boolean {
  return (
    matchesPath(TRAEGER_SETTINGS_HOME, pathname) ||
    matchesPath(TRAEGER_SETTINGS_HOME_ON_HOST, pathname)
  );
}

/**
 * Die drei Bereiche der Abrechnung teilen sich die Adressen unter
 * /abrechnung. Familien und Preise nennen ihre Register; alles andere unter
 * /abrechnung (die Ferienläufe, die Monatsbeiträge, ein Lauf) gehört zu den
 * Abrechnungsläufen.
 */
const BILLING_AREA_REGISTERS: Readonly<Record<string, readonly string[]>> = {
  "/traeger/abrechnung/familien": [
    "familien",
    "lastschrift",
    "kinderzahl",
    "nachweise",
    "kostentraeger",
    "exporte",
  ],
  "/traeger/abrechnung/preise": ["preise", "mensa", "ermaessigungen"],
};
const BILLING_HOME = "/traeger/abrechnung";

/** Das erste Segment nach /abrechnung, mit oder ohne /traeger davor. */
function billingRegister(pathname: string): string | null {
  const page = pathname.replace(/^\/traeger(?=\/|$)/, "");
  if (page !== "/abrechnung" && !page.startsWith("/abrechnung/")) return null;
  return page.slice("/abrechnung".length).split("/")[1] ?? "";
}

function isBillingAreaActive(href: string, pathname: string): boolean | null {
  const registers = BILLING_AREA_REGISTERS[href];
  if (href !== BILLING_HOME && !registers) return null;
  const register = billingRegister(pathname);
  if (register === null) return false;
  if (registers) return registers.includes(register);
  return !Object.values(BILLING_AREA_REGISTERS).some((list) =>
    list.includes(register),
  );
}

export function isTraegerNavActive(href: string, pathname: string): boolean {
  const billing = isBillingAreaActive(href, pathname);
  if (billing !== null) return billing;
  if (href === "/traeger") {
    // „Schulen" ist die Startseite; ihre Unterseiten liegen unter /schulen.
    return (
      pathname === "/traeger" ||
      pathname === "/" ||
      matchesPath("/schulen", pathname) ||
      matchesPath("/traeger/schulen", pathname)
    );
  }
  if (href === TRAEGER_SETTINGS_HOME) {
    // „Allgemein" ist die Startseite der Einstellungen; deren Unterseiten sind
    // eigene Einträge.
    return (
      pathname === TRAEGER_SETTINGS_HOME ||
      pathname === TRAEGER_SETTINGS_HOME_ON_HOST
    );
  }
  if (matchesPath(href, pathname)) return true;
  if (href.startsWith("/traeger/")) {
    return matchesPath(href.slice("/traeger".length), pathname);
  }
  return false;
}

function matchesPath(href: string, pathname: string): boolean {
  return pathname === href || pathname.startsWith(`${href}/`);
}
