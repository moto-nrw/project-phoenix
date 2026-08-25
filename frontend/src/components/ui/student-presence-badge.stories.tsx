import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import type { TenantInfo } from "~/lib/tenant-api";
import { TenantProvider } from "~/lib/tenant-context";
import type { StudentLocationContext } from "@/lib/location-helper";
import { StudentPresenceBadge } from "./student-presence-badge";

const meta = {
  title: "components/ui/StudentPresenceBadge",
  component: StudentPresenceBadge,
} satisfies Meta<typeof StudentPresenceBadge>;

export default meta;

type Story = StoryObj<typeof meta>;

const presentStudent: StudentLocationContext = {
  current_location: "Raum 101",
  location_since: "2026-07-01T09:15:00.000Z",
  group_name: "Gruppe A",
};

export const Detailed: Story = {
  args: {
    student: presentStudent,
    displayMode: "roomName",
  },
};

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

export const Binary: Story = {
  args: {
    student: presentStudent,
    displayMode: "roomName",
  },
  decorators: [
    (StoryFn) => (
      <TenantProvider
        tenantSlug="storybook"
        tenant={binaryTenant}
        routingMode="path"
      >
        <StoryFn />
      </TenantProvider>
    ),
  ],
};
