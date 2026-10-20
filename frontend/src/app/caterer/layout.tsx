import { CatererProviders } from "./providers";
import { CatererAuthGuard } from "./auth-guard";

/**
 * Server layout of the caterer portal (ADR 0075): the caterer session, then
 * the client-side guard. German-only like the school portal.
 */
export default function CatererLayout({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  return (
    <CatererProviders>
      <CatererAuthGuard>{children}</CatererAuthGuard>
    </CatererProviders>
  );
}
