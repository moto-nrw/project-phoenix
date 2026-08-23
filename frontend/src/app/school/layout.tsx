import { SchoolProviders } from "./providers";
import { SchoolAuthGuard } from "./auth-guard";

/**
 * Server layout for school routes ("moto schule", #2207).
 * Wraps children in SchoolProviders (SessionProvider with school basePath),
 * then SchoolAuthGuard handles client-side auth checks.
 *
 * German-only like the staff portal — no NextIntlClientProvider here.
 */
export default function SchoolLayout({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  return (
    <SchoolProviders>
      <SchoolAuthGuard>{children}</SchoolAuthGuard>
    </SchoolProviders>
  );
}
