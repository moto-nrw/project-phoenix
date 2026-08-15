import type { MetadataRoute } from "next";

/**
 * Manifest der Eltern-App. Eigenes Manifest statt des geteilten Wurzel-
 * Manifests (app/manifest.ts, Name "MOTO"), weil Name, Beschreibung und
 * Ausrichtung eltern-spezifisch sind: Eltern installieren "moto Eltern".
 *
 * Ohne verlinktes Manifest liefert "Zum Home-Bildschirm" auf iOS nur ein
 * Lesezeichen, die App startet nicht eigenstaendig, und Web Push funktioniert
 * dort gar nicht.
 *
 * Startadresse und Geltungsbereich bilden die OEFFENTLICHE Sicht ab: der Proxy
 * (src/proxy.ts) rewritet den Eltern-Host intern auf /parents/*, oeffentlich
 * beginnen die Adressen ohne dieses Praefix. Ein Geltungsbereich "/parents"
 * wuerde deshalb keine einzige echte Adresse enthalten und die Installation
 * verhindern.
 *
 * Ausgeliefert wird das Ergebnis von app/parents/manifest.webmanifest/route.ts.
 * Next.js erkennt die Manifest-Dateikonvention nur im Wurzelverzeichnis von
 * app/ (die Regex in next/dist/lib/metadata/is-metadata-route.js ist auf den
 * Anfang des Pfades verankert), deshalb der ausdrueckliche Route Handler.
 */
/** Symbolsatz der Eltern-App: Produktion und Staging sind unterscheidbar. */
export type ParentsIconVariant = "eltern" | "eltern-staging";

export default function parentsManifest(
  iconVariant: ParentsIconVariant = "eltern",
): MetadataRoute.Manifest {
  const icons = `/favicons/${iconVariant}`;
  return {
    name: "moto Eltern",
    short_name: "moto",
    description:
      "Die Betreuung Ihres Kindes im Blick: Tagesstatus, Nachrichten an die OGS, Krankmeldung und Termine.",
    lang: "de",
    dir: "ltr",
    display: "standalone",
    orientation: "portrait-primary",
    start_url: "/",
    scope: "/",
    background_color: "#ffffff",
    theme_color: "#ffffff",
    categories: ["education"],
    icons: [
      {
        src: `${icons}/icon-192.png`,
        sizes: "192x192",
        type: "image/png",
        purpose: "any",
      },
      {
        src: `${icons}/icon-512.png`,
        sizes: "512x512",
        type: "image/png",
        purpose: "any",
      },
      {
        src: `${icons}/icon-maskable-192.png`,
        sizes: "192x192",
        type: "image/png",
        purpose: "maskable",
      },
      {
        src: `${icons}/icon-maskable-512.png`,
        sizes: "512x512",
        type: "image/png",
        purpose: "maskable",
      },
    ],
  };
}
