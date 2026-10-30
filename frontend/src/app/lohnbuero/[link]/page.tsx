import type { Metadata } from "next";

import { PayrollDownloadContent } from "./payroll-download-content";

// Öffentliche Seite hinter dem Link aus der E-Mail an das Lohnbüro (#4805).
// Außerhalb aller Portale: kein Konto, keine Schule im Host. Die Seite selbst
// verbraucht den Link nicht; erst der Knopf holt die Datei per POST.
export const metadata: Metadata = {
  title: "Monatsdatei für das Lohnbüro – moto",
  robots: { index: false, follow: false },
  // Der Link ist die Berechtigung und gehört in keinen Referer.
  referrer: "no-referrer",
};

export default async function PayrollDownloadPage({
  params,
}: {
  params: Promise<{ link: string }>;
}) {
  const { link } = await params;
  return <PayrollDownloadContent link={link} />;
}
