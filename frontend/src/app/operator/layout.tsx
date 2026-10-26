import { OperatorProviders } from "./providers";
import { OperatorAuthGuard } from "./auth-guard";
import { readOperatorSessionSnapshot } from "~/lib/operator-session-snapshot.server";

/**
 * Server layout for operator routes.
 * Wraps children in OperatorProviders (SessionProvider with operator basePath),
 * then OperatorAuthGuard handles client-side auth checks.
 *
 * A valid portal cookie hydrates SessionProvider through a read-only
 * snapshot (#4991), so the guard renders the page without first fetching
 * the session. Never call the portal's auth() here: its refresh callback
 * could rotate the token in a Server Component that cannot persist the
 * new cookie (#1938).
 */
export default async function OperatorLayout({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  const session = await readOperatorSessionSnapshot();
  return (
    <OperatorProviders session={session}>
      <OperatorAuthGuard>{children}</OperatorAuthGuard>
    </OperatorProviders>
  );
}
