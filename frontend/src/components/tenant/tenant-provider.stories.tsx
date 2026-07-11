import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { TenantProvider, useTenant } from "~/lib/tenant-context";
import type { TenantInfo } from "~/lib/tenant-api";

const fakeTenant: TenantInfo = {
  tenantId: 1,
  slug: "storybook",
  name: "Storybook Schule",
  subdomain: "storybook",
  organizationId: 1,
  organizationName: "Storybook Träger",
  settings: {},
  presenceMode: "detailed",
  studentPhotosEnabled: true,
  nfcEnabled: false,
  messagingEnabled: false,
  displayEnabled: false,
  gradeLevelMax: 4,
};

function TenantSummary() {
  const { tenantSlug, tenant, routingMode } = useTenant();
  return (
    <dl className="space-y-1 text-sm">
      <div>
        <dt className="inline font-semibold">tenantSlug: </dt>
        <dd className="inline">{tenantSlug}</dd>
      </div>
      <div>
        <dt className="inline font-semibold">routingMode: </dt>
        <dd className="inline">{routingMode}</dd>
      </div>
      <div>
        <dt className="inline font-semibold">tenant.name: </dt>
        <dd className="inline">{tenant?.name ?? "null"}</dd>
      </div>
    </dl>
  );
}

const meta: Meta<typeof TenantProvider> = {
  title: "components/tenant/TenantProvider",
  component: TenantProvider,
};

export default meta;

type Story = StoryObj<typeof TenantProvider>;

export const WithResolvedTenant: Story = {
  render: () => (
    <TenantProvider
      tenantSlug="storybook"
      tenant={fakeTenant}
      routingMode="path"
    >
      <TenantSummary />
    </TenantProvider>
  ),
};

export const SubdomainRoutingWithoutResolvedTenant: Story = {
  render: () => (
    <TenantProvider
      tenantSlug="storybook"
      tenant={null}
      routingMode="subdomain"
    >
      <TenantSummary />
    </TenantProvider>
  ),
};
