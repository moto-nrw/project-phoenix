import { TraegerProviders } from "./providers";
import { TraegerAuthGuard } from "./auth-guard";

/**
 * Server layout for Träger routes ("moto träger", #4649). German-only like
 * the staff portals — no NextIntlClientProvider here.
 */
export default function TraegerLayout({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  return (
    <TraegerProviders>
      <TraegerAuthGuard>{children}</TraegerAuthGuard>
    </TraegerProviders>
  );
}
