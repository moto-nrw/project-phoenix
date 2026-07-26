import "~/styles/globals.css";

import { Providers } from "./providers";
import { BackgroundWrapper } from "~/components/background-wrapper";
import { Inter, Kalam } from "next/font/google";
import type { Metadata } from "next";
import { headers } from "next/headers";
import { getLocale } from "next-intl/server";
import { faviconMetadata, resolveFaviconVariant } from "~/lib/favicon-variants";

const inter = Inter({ subsets: ["latin"] });
const motoFont = Kalam({
  subsets: ["latin"],
  weight: "700",
  display: "swap",
  preload: true,
  variable: "--font-moto",
});

const baseMetadata = {
  title: "moto – Digitale Ganztagsbetreuung",
  description:
    "Das innovative An- und Abmeldesystem mit NFC-Armbändern für die offene Ganztagsschule. DSGVO-konform, entwickelt an der Universität Münster.",
} satisfies Metadata;

function requiredEnv(name: string): string {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is not set.`);
  return value;
}

export async function generateMetadata(): Promise<Metadata> {
  const requestHeaders = await headers();
  const host =
    requestHeaders.get("x-forwarded-host") ?? requestHeaders.get("host") ?? "";
  const variant = resolveFaviconVariant(host, {
    operatorHostname: requiredEnv("NEXT_PUBLIC_OPERATOR_HOSTNAME"),
    parentsHostname: requiredEnv("NEXT_PUBLIC_PARENTS_HOSTNAME"),
    tenantDomain: requiredEnv("TENANT_DOMAIN"),
  });

  return {
    ...baseMetadata,
    ...faviconMetadata(variant),
  };
}

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
      <body className={`font-sans ${inter.className} ${motoFont.variable}`}>
        <Providers>
          <BackgroundWrapper>{children}</BackgroundWrapper>
        </Providers>
      </body>
    </html>
  );
}
