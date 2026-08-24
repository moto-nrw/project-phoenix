/**
 * Welches Navigationsziel zur aktuellen Adresse gehört.
 *
 * Auf dem Schul-Host schreibt der Proxy "/" intern auf /school um, die
 * Adresszeile zeigt also "/" statt "/school" und "/aufsichten" statt
 * "/school/aufsichten". Beide Schreibweisen müssen dasselbe Ziel markieren,
 * sonst leuchtet auf dem echten Host nichts. Gleiches Problem und gleiche
 * Lösung wie `parent-nav-active.ts`.
 */
export function isSchoolNavActive(href: string, pathname: string): boolean {
  if (href === "/school") {
    return pathname === "/school" || pathname === "/";
  }

  if (matchesPath(href, pathname)) return true;

  // Portal-Unterseiten ohne /school-Präfix, wie der Schul-Host sie zeigt.
  if (href.startsWith("/school/")) {
    return matchesPath(href.slice("/school".length), pathname);
  }

  return false;
}

function matchesPath(href: string, pathname: string): boolean {
  return pathname === href || pathname.startsWith(`${href}/`);
}
