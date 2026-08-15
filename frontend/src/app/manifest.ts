import type { MetadataRoute } from "next";
import { headers } from "next/headers";
import {
  faviconManifest,
  isParentsHost,
  resolveFaviconVariant,
} from "~/lib/favicon-variants";

function requiredEnv(name: string): string {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is not set.`);
  return value;
}

/**
 * Eltern-spezifische Angaben für das Manifest.
 *
 * Das Manifest MUSS hier entstehen und nicht als eigene Route unter
 * /parents: der Proxy bildet den Eltern-Host auf /parents/* ab, öffentliche
 * URLs beginnen dort also ohne /parents. Eine Route unter /parents wäre für
 * den Browser nur über eine 307-Umleitung erreichbar, und ein Manifest hinter
 * einer Umleitung macht die App nicht installierbar.
 *
 * Ohne eigenständige Installation gibt es auf iOS keine
 * Web-Push-Benachrichtigungen, deshalb hängt daran mehr als nur der Name auf
 * dem Home-Bildschirm.
 */
const PARENTS_MANIFEST = {
  name: "moto Eltern",
  short_name: "moto",
  description:
    "Die Betreuung Ihres Kindes im Blick: Tagesstatus, Nachrichten an die OGS, Krankmeldung und Termine.",
  orientation: "portrait-primary",
  lang: "de",
} satisfies Partial<MetadataRoute.Manifest>;

export default async function manifest(): Promise<MetadataRoute.Manifest> {
  const requestHeaders = await headers();
  const host =
    requestHeaders.get("x-forwarded-host") ?? requestHeaders.get("host") ?? "";
  const config = {
    operatorHostname: requiredEnv("NEXT_PUBLIC_OPERATOR_HOSTNAME"),
    parentsHostname: requiredEnv("NEXT_PUBLIC_PARENTS_HOSTNAME"),
    tenantDomain: requiredEnv("TENANT_DOMAIN"),
  };

  const base = faviconManifest(resolveFaviconVariant(host, config));

  // Die Symbole kommen weiterhin aus der host-aufgelösten Favicon-Variante,
  // nur Name, Beschreibung und Ausrichtung sind für Eltern eigene. Der
  // Eltern-Host wird direkt erkannt, nicht über die Favicon-Variante: die ist
  // dort bewusst "normal".
  if (isParentsHost(host, config)) {
    return { ...base, ...PARENTS_MANIFEST };
  }

  return base;
}
