// Form des Download-Links für das Lohnbüro (#4805): „<Schul-ID>.<Token>“.
// Das Token ist base64url ohne Auffüllung (32 Zufallsbytes, 43 Zeichen); die
// Grenzen lassen dem Backend Spielraum, ohne Beliebiges durchzureichen.
const PAYROLL_DOWNLOAD_LINK = /^\d{1,19}\.[A-Za-z0-9_-]{20,128}$/;

/** Ob ein Pfadstück die Form eines Download-Links hat. */
export function isPayrollDownloadLink(link: string): boolean {
  return PAYROLL_DOWNLOAD_LINK.test(link);
}
