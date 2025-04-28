import "~/styles/globals.css";

import { SessionProvider } from "~/components/session-provider";
import { BackgroundWrapper } from "~/components/background-wrapper";
import { Inter } from "next/font/google";

const inter = Inter({ subsets: ["latin"] });

export const metadata = {
  title: "Project Phoenix",
  description: "A modern full-stack application",
  icons: [
    { rel: "icon", url: "/favicon.png", type: "image/png" },
    { rel: "apple-touch-icon", url: "/favicon.png" }
  ],
  manifest: "/site.webmanifest",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body className={`font-sans ${inter.className}`}>
        <SessionProvider>
          <BackgroundWrapper>{children}</BackgroundWrapper>
        </SessionProvider>
      </body>
    </html>
  );
}