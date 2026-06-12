import "~/styles/globals.css";

import { Providers } from "./providers";
import { BackgroundWrapper } from "~/components/background-wrapper";
import { Inter } from "next/font/google";
import { getLocale } from "next-intl/server";

const inter = Inter({ subsets: ["latin"] });

export const metadata = {
  title: "moto – Digitale Ganztagsbetreuung",
  description:
    "Das innovative An- und Abmeldesystem mit NFC-Armbändern für die offene Ganztagsschule. DSGVO-konform, entwickelt an der Universität Münster.",
  icons: [
    { rel: "icon", url: "/favicon.png", type: "image/png" },
    { rel: "apple-touch-icon", url: "/apple-touch-icon.png", sizes: "180x180" },
  ],
  manifest: "/site.webmanifest",
};

export const viewport = {
  width: "device-width",
  initialScale: 1,
  maximumScale: 1,
  userScalable: false,
};

export default async function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  // Resolve the locale for <html lang> only. request.ts returns the default
  // locale ("de") on every surface except the parent-facing ones (the proxy
  // sets the localize header there), so staff/operator stay German. The
  // NextIntlClientProvider — and the message catalog it serializes to the
  // client — is mounted only on the localized surfaces (parents layout +
  // public enrollment layout), not app-wide, so the German-only portals don't
  // ship the parent catalog.
  const locale = await getLocale();
  return (
    <html lang={locale}>
      <body className={`font-sans ${inter.className}`}>
        <Providers>
          <BackgroundWrapper>{children}</BackgroundWrapper>
        </Providers>
      </body>
    </html>
  );
}
