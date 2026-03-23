import "~/styles/globals.css";

import { Providers } from "./providers";
import { BackgroundWrapper } from "~/components/background-wrapper";
import { Inter } from "next/font/google";

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

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="de">
      <body className={`font-sans ${inter.className}`}>
        <Providers>
          <BackgroundWrapper>{children}</BackgroundWrapper>
        </Providers>
      </body>
    </html>
  );
}
