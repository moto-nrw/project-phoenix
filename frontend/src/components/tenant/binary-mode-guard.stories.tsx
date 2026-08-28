import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { TenantProvider } from "~/lib/tenant-context";
import type { TenantInfo } from "~/lib/tenant-api";
import { BinaryModeGuard } from "./binary-mode-guard";

const binaryTenant: TenantInfo = {
  tenantId: 1,
  slug: "storybook",
  name: "Storybook Schule",
  subdomain: "storybook",
  organizationId: 1,
  organizationName: "Storybook Org",
  settings: {},
  presenceMode: "binary",
  studentPhotosEnabled: true,
  nfcEnabled: true,
  messagingEnabled: true,
  staffMessagingEnabled: false,
  displayEnabled: false,
  gradeLevelMax: 4,
};

const meta: Meta<typeof BinaryModeGuard> = {
  title: "components/tenant/BinaryModeGuard",
  component: BinaryModeGuard,
};

export default meta;

type Story = StoryObj<typeof BinaryModeGuard>;

/**
 * Detailed-mode tenant (the global decorator's default tenant): the guard
 * lets the children render through untouched.
 */
export const DetailedModePassesThrough: Story = {
  args: {
    children: <div>Geschützter Seiteninhalt</div>,
  },
};

/**
 * Binary-mode tenant: the guard calls `notFound()`, which Next.js surfaces
 * as a thrown navigation signal. Storybook renders this as an error state,
 * which is the expected defense-in-depth behaviour for direct URL entry.
 */
export const BinaryModeTriggersNotFound: Story = {
  args: {
    children: <div>Geschützter Seiteninhalt</div>,
  },
  decorators: [
    (Story) => (
      <TenantProvider
        tenantSlug="storybook"
        tenant={binaryTenant}
        routingMode="path"
      >
        <Story />
      </TenantProvider>
    ),
  ],
};
