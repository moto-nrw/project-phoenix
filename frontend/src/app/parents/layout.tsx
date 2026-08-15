import type { Metadata } from "next";
import { NextIntlClientProvider } from "next-intl";
import { ParentProviders } from "./providers";
import { ParentAuthGuard } from "./auth-guard";

/**
 * Erlaubt iOS, die Eltern-App ausserhalb des Browser-Rahmens zu starten.
 *
 * Das Manifest selbst wird NICHT hier verlinkt: der Proxy bildet den
 * Eltern-Host auf /parents/* ab, oeffentliche URLs beginnen dort also ohne
 * /parents. Ein Verweis auf /parents/manifest.webmanifest liefe in eine
 * 307-Umleitung, und hinter einer Umleitung ist eine App nicht installierbar.
 * Stattdessen erkennt app/manifest.ts den Eltern-Host und liefert unter dem
 * bereits verlinkten /manifest.webmanifest die eltern-spezifischen Angaben.
 *
 * Ohne appleWebApp oeffnet iOS die Seite trotz Manifest im Browser-Rahmen, und
 * ohne eigenstaendigen Start gibt es dort keine Web-Push-Benachrichtigungen.
 */
export const metadata: Metadata = {
  appleWebApp: {
    capable: true,
    statusBarStyle: "default",
    title: "moto Eltern",
  },
};

/**
 * Server layout for parent routes.
 * Wraps children in ParentProviders (SessionProvider with parent basePath),
 * then ParentAuthGuard handles client-side auth checks.
 *
 * The whole parents subtree is localized, so it carries the full
 * NextIntlClientProvider. Rendered in a Server Component, it auto-inherits the
 * resolved locale + message catalog from request.ts (the proxy flags the
 * parents subdomain as localized). This is the only place — together with the
 * public enrollment layout — that ships the parent message catalog to the
 * client; the German-only staff/operator portals never mount it.
 */
export default function ParentLayout({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  return (
    <NextIntlClientProvider>
      <ParentProviders>
        <ParentAuthGuard>{children}</ParentAuthGuard>
      </ParentProviders>
    </NextIntlClientProvider>
  );
}
