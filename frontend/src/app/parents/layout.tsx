import type { Metadata } from "next";
import { NextIntlClientProvider } from "next-intl";
import { ParentProviders } from "./providers";
import { ParentAuthGuard } from "./auth-guard";

/**
 * Verweist die Eltern-App auf ihr eigenes Manifest und erlaubt iOS den Start
 * ausserhalb des Browser-Rahmens.
 *
 * Die Wurzel (app/layout.tsx) verlinkt /manifest.webmanifest mit dem Namen
 * "MOTO". Diese Angaben ueberschreiben das nur unterhalb von /parents, das
 * Personal- und das Operator-Portal bleiben unberuehrt.
 *
 * Ohne appleWebApp oeffnet iOS die Seite trotz Manifest im Browser-Rahmen, und
 * ohne eigenstaendigen Start gibt es dort keine Web-Push-Benachrichtigungen.
 */
export const metadata: Metadata = {
  manifest: "/parents/manifest.webmanifest",
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
