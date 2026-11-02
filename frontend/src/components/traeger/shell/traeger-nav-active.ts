/**
 * Welches Navigationsziel zur aktuellen Adresse gehört.
 *
 * Auf dem Träger-Host schreibt der Proxy "/" intern auf /traeger um, die
 * Adresszeile zeigt also "/ferien" statt "/traeger/ferien". Beide
 * Schreibweisen markieren dasselbe Ziel. Gleiches Problem und gleiche Lösung
 * wie `school/shell/school-nav-active.ts`.
 */
export function isTraegerNavActive(href: string, pathname: string): boolean {
  if (href === "/traeger") {
    // „Schulen" ist die Startseite und hat keine Unterseiten.
    return pathname === "/traeger" || pathname === "/";
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
