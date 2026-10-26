import { CatererProviders } from "./providers";
import { CatererAuthGuard } from "./auth-guard";
import { readCatererSessionSnapshot } from "~/lib/caterer-session-snapshot.server";

/**
 * Server layout of the caterer portal (ADR 0077): the caterer session, then
 * the client-side guard. German-only like the school portal.
 *
 * A valid portal cookie hydrates SessionProvider through a read-only
 * snapshot (#4991), so the guard renders the page without first fetching
 * the session. Never call the portal's auth() here: its refresh callback
 * could rotate the token in a Server Component that cannot persist the
 * new cookie (#1938).
 */
export default async function CatererLayout({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  const session = await readCatererSessionSnapshot();
  return (
    <CatererProviders session={session}>
      <CatererAuthGuard>{children}</CatererAuthGuard>
    </CatererProviders>
  );
}
