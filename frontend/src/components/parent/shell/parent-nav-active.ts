/**
 * Welches Navigationsziel zur aktuellen Adresse gehoert.
 *
 * Auf dem Eltern-Host schreibt der Proxy /parents/* intern um, die Adresszeile
 * zeigt also /messages statt /parents/messages. Beide Schreibweisen muessen
 * dasselbe Ziel markieren, sonst leuchtet auf dem echten Host nichts.
 */
export function isParentNavActive(href: string, pathname: string): boolean {
  const bare = href.replace(/^\/parents/, "") || "/";

  if (href === "/parents") {
    return pathname === "/parents" || pathname === "/";
  }

  return (
    pathname === href ||
    pathname.startsWith(`${href}/`) ||
    pathname === bare ||
    pathname.startsWith(`${bare}/`)
  );
}
