import "~/styles/globals.css";

import { Providers } from "./providers";
import { BackgroundWrapper } from "~/components/background-wrapper";
import { Inter } from "next/font/google";

const inter = Inter({ subsets: ["latin"] });

export const metadata = {
  title: "moto",
  description: "A modern full-stack application",
  icons: [
    { rel: "icon", url: "/favicon.png", type: "image/png" },
    { rel: "apple-touch-icon", url: "/favicon.png" },
  ],
  manifest: "/site.webmanifest",
  viewport: {
    width: "device-width",
    initialScale: 1,
    maximumScale: 1,
    userScalable: false,
  },
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body className={`font-sans ${inter.className}`}>
        <Providers>
          <BackgroundWrapper>{children}</BackgroundWrapper>
        </Providers>
      </body>
    </html>
  );
}
