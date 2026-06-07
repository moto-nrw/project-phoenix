import { ParentProviders } from "./providers";
import { ParentAuthGuard } from "./auth-guard";

/**
 * Server layout for parent routes.
 * Wraps children in ParentProviders (SessionProvider with parent basePath),
 * then ParentAuthGuard handles client-side auth checks.
 */
export default function ParentLayout({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  return (
    <ParentProviders>
      <ParentAuthGuard>{children}</ParentAuthGuard>
    </ParentProviders>
  );
}
