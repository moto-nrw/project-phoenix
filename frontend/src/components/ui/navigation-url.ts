/**
 * Adressen für Links und Seitenwechsel (#6570): Ziel, aktuelle Seite und die
 * Prüfung „Pfad dieser Seite“ an einer Stelle. `NavigationLink`, die
 * Fortschrittsanzeige und der Rückweg aus den Einstellungen nutzen sie.
 */

/**
 * Ein Pfad dieser Seite. "//host" und "/\host" lösen Browser zu einer fremden
 * Adresse auf.
 */
export function isInternalPath(href: string): boolean {
  return (
    href.startsWith("/") && !href.startsWith("//") && !href.startsWith("/\\")
  );
}

/** Pfad und Query in einer Form, die Router-Commit und Link vergleichbar macht. */
export function normalizedUrl(pathname: string, search: string): string {
  const normalizedSearch = new URLSearchParams(search).toString();
  return normalizedSearch === "" ? pathname : `${pathname}?${normalizedSearch}`;
}

/** Die aktuelle Seite in der Form von `normalizedUrl`. */
export function currentUrl(): string {
  return normalizedUrl(window.location.pathname, window.location.search);
}

/** Das Ziel eines Links auf dieser Seite, sonst `null`. */
export function navigationTarget(href: string): string | null {
  try {
    const target = new URL(href, window.location.href);
    if (target.origin !== window.location.origin) return null;
    return normalizedUrl(target.pathname, target.search);
  } catch {
    return null;
  }
}
