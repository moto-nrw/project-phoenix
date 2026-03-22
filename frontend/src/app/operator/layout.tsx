import { OperatorProviders } from "./providers";
import { OperatorAuthGuard } from "./auth-guard";

/**
 * Server layout for operator routes.
 * Wraps children in OperatorProviders (SessionProvider with operator basePath),
 * then OperatorAuthGuard handles client-side auth checks.
 */
export default function OperatorLayout({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  return (
    <OperatorProviders>
      <OperatorAuthGuard>{children}</OperatorAuthGuard>
    </OperatorProviders>
  );
}
