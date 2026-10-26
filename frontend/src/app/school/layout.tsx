import { SchoolProviders } from "./providers";
import { SchoolAuthGuard } from "./auth-guard";
import { readSchoolSessionSnapshot } from "~/lib/school-session-snapshot.server";

/**
 * Server layout for school routes ("moto schule", #2207).
 * Wraps children in SchoolProviders (SessionProvider with school basePath),
 * then SchoolAuthGuard handles client-side auth checks.
 *
 * German-only like the staff portal — no NextIntlClientProvider here.
 *
 * A valid portal cookie hydrates SessionProvider through a read-only
 * snapshot (#4991), so the guard renders the page without first fetching
 * the session. Never call the portal's auth() here: its refresh callback
 * could rotate the token in a Server Component that cannot persist the
 * new cookie (#1938).
 */
export default async function SchoolLayout({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  const session = await readSchoolSessionSnapshot();
  return (
    <SchoolProviders session={session}>
      <SchoolAuthGuard>{children}</SchoolAuthGuard>
    </SchoolProviders>
  );
}
