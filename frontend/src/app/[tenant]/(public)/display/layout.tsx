/**
 * Public info-point display surface (issue #1325). Unlike /enroll this is a
 * staff-facing big-screen page and stays German-only: no NextIntlClientProvider,
 * and /display is deliberately NOT in the proxy's localized-path list.
 */
export default function DisplayLayout({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  return <>{children}</>;
}
