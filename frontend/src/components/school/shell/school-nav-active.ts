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
  const queryStart = href.indexOf("?");
  const hrefPathname = queryStart === -1 ? href : href.slice(0, queryStart);

  if (hrefPathname === "/school") {
    // Die Klassenseite (#2294) liegt unter der Klassenansicht: ohne sie hier
    // stünde man auf einer Unterseite und die Navigation zeigte nirgends hin.
    return (
      pathname === "/school" ||
      pathname === "/" ||
      matchesPath("/klasse", pathname) ||
      matchesPath("/school/klasse", pathname)
    );
  }

  if (matchesPath(hrefPathname, pathname)) return true;

  // Portal-Unterseiten ohne /school-Präfix, wie der Schul-Host sie zeigt.
  if (hrefPathname.startsWith("/school/")) {
    return matchesPath(hrefPathname.slice("/school".length), pathname);
  }

  return false;
}

function matchesPath(href: string, pathname: string): boolean {
  return pathname === href || pathname.startsWith(`${href}/`);
}
