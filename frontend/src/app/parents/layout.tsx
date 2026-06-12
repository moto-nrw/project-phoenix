import { NextIntlClientProvider } from "next-intl";
import { ParentProviders } from "./providers";
import { ParentAuthGuard } from "./auth-guard";

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
