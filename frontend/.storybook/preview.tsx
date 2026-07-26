import type { Preview } from "@storybook/nextjs-vite";
import { SessionProvider } from "next-auth/react";
import { NextIntlClientProvider } from "next-intl";
import type { AbstractIntlMessages } from "next-intl";
import { Providers } from "~/app/providers";
import { TenantProvider } from "~/lib/tenant-context";
import { mockSessionData } from "./fixtures/session";
import type { TenantInfo } from "~/lib/tenant-api";
import deMessages from "~/i18n/messages/de.json";

import "~/styles/globals.css";

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
