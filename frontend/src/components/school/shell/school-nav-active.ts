/**
 * Welches Navigationsziel zur aktuellen Adresse gehört.
 *
 * Auf dem Schul-Host schreibt der Proxy "/" intern auf /school um, die
 * Adresszeile zeigt also "/" statt "/school". Beide Schreibweisen müssen
 * dasselbe Ziel markieren, sonst leuchtet auf dem echten Host nichts.
 * Gleiches Problem und gleiche Lösung wie `parent-nav-active.ts`.
 */
export function isSchoolNavActive(href: string, pathname: string): boolean {
  if (href === "/school") {
    return pathname === "/school" || pathname === "/";
  }

  return pathname === href || pathname.startsWith(`${href}/`);
}
