import { TraegerProviders } from "./providers";
import { TraegerAuthGuard } from "./auth-guard";
import { readPortalSessionSnapshot } from "~/lib/portal-session-snapshot.server";

/**
 * Server layout for Träger routes ("moto träger", #4649). German-only like
 * the staff portals — no NextIntlClientProvider here.
 *
 * A valid portal cookie hydrates SessionProvider through a read-only
 * snapshot (#4991), so the guard renders the page without first fetching
 * the session. Never call the portal's auth() here: its refresh callback
 * could rotate the token in a Server Component that cannot persist the
 * new cookie (#1938).
 */
export default async function TraegerLayout({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  const session = (await readPortalSessionSnapshot("traeger"))?.session ?? null;
  return (
    <TraegerProviders session={session}>
      <TraegerAuthGuard>{children}</TraegerAuthGuard>
    </TraegerProviders>
  );
}
