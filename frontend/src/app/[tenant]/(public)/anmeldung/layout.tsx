import { NextIntlClientProvider } from "next-intl";

/**
 * Public enrollment is the only parent-facing surface on a tenant subdomain,
 * so it is localized while the rest of the tenant (staff) portal stays German.
 * The proxy flags /anmeldung/* as localized; this Server Component provider
 * auto-inherits the resolved locale + message catalog from request.ts.
 *
 * Scoping the provider here (rather than app-wide in the root layout) keeps the
 * parent message catalog out of every staff/operator page bundle. The staff
 * enrollment preview lives under this layout too and opts INTO localized copy
 * (EnrollmentForm's `localizedCopy`) so its inline language switcher actually
 * previews the form in each parent locale; the catalog is already mounted here.
 */
export default function EnrollLayout({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  return <NextIntlClientProvider>{children}</NextIntlClientProvider>;
}
