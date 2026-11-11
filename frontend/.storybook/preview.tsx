import type { Preview } from "@storybook/nextjs-vite";
import { SessionProvider } from "next-auth/react";
import { NextIntlClientProvider } from "next-intl";
import type { AbstractIntlMessages } from "next-intl";
import { Providers } from "~/app/providers";
import { TenantProvider } from "~/lib/tenant-context";
import { mockSessionData } from "./fixtures/session";
import type { TenantInfo } from "~/lib/tenant-api";
import deMessages from "~/i18n/messages/de.generated";

import "~/styles/globals.css";
import "./fonts.css";

// Fake tenant metadata satisfying TenantInfo (see ~/lib/tenant-api) so
// components reading tenant context (LOCATION_COLORS-driven badges, feature
// gates, etc.) render as if inside the real [tenant] route segment.
const storybookTenant: TenantInfo = {
  tenantId: 1,
  slug: "storybook",
  name: "Storybook Schule",
  subdomain: "storybook",
  organizationId: 1,
  organizationName: "Storybook Org",
  settings: {},
  presenceMode: "detailed",
  studentPhotosEnabled: true,
  nfcEnabled: true,
  messagingEnabled: true,
};

const preview: Preview = {
  // Schriften vor jedem Render laden, wie next/font es in der App vorab tut.
  // Sonst misst ein Baustein beim Mounten mit der Ersatzschrift und misst
  // nicht neu: der Unterstrich von NavigationTabs saß im Pixel-Vergleich
  // (#6562) in etwa jedem fünften Lauf 18 px zu weit links.
  loaders: [
    async () => {
      await Promise.all([
        document.fonts.load('1em "Inter Storybook"'),
        document.fonts.load('700 1em "Kalam Storybook"'),
      ]);
      return {};
    },
  ],
  parameters: {
    a11y: {
      test: "todo",
    },
    controls: {
      matchers: {
        color: /(background|color)$/i,
        date: /Date$/i,
      },
    },
    nextjs: {
      appDirectory: true,
    },
  },
  decorators: [
    (Story) => (
      <SessionProvider session={mockSessionData()}>
        <NextIntlClientProvider
          locale="de"
          messages={deMessages as AbstractIntlMessages}
        >
          <TenantProvider
            tenantSlug="storybook"
            tenant={storybookTenant}
            routingMode="path"
          >
            <Providers>
              <Story />
            </Providers>
          </TenantProvider>
        </NextIntlClientProvider>
      </SessionProvider>
    ),
  ],
};

export default preview;
