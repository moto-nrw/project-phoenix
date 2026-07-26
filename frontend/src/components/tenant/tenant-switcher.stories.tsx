import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { installStorybookFetch, jsonResponse } from "~storybook/mocks/fetch";
import { TenantSwitcher } from "./tenant-switcher";

/** Minimal shape returned by the backend's `/api/auth/account-tenants` endpoint. */
interface AccountTenantBackendFixture {
  tenant_id: number;
  slug: string;
  name: string;
  subdomain: string;
  organization_id: number;
  organization_name: string;
}

const singleTenant: AccountTenantBackendFixture[] = [
  {
    tenant_id: 1,
    slug: "storybook",
    name: "Storybook Schule",
    subdomain: "storybook",
    organization_id: 1,
    organization_name: "Storybook Org",
  },
];

const sameOrgTenants: AccountTenantBackendFixture[] = [
  ...singleTenant,
  {
    tenant_id: 2,
    slug: "nord",
    name: "OGS Nord",
    subdomain: "nord",
    organization_id: 1,
    organization_name: "Storybook Org",
  },
  {
    tenant_id: 3,
    slug: "west",
    name: "OGS West",
    subdomain: "west",
    organization_id: 1,
    organization_name: "Storybook Org",
  },
];

const multiOrgTenants: AccountTenantBackendFixture[] = [
  ...sameOrgTenants,
  {
    tenant_id: 4,
    slug: "sued",
    name: "OGS Süd",
    subdomain: "sued",
    organization_id: 2,
    organization_name: "Andere Trägerschaft",
  },
];

function mockAccountTenants(tenants: AccountTenantBackendFixture[]) {
  return installStorybookFetch(({ url }) => {
    if (!url.includes("/api/auth/account-tenants")) return undefined;
    return jsonResponse({ data: tenants });
  });
}

const meta = {
  title: "tenant/TenantSwitcher",
  component: TenantSwitcher,
  parameters: {
    docs: {
      description: {
        component:
          "Only renders once `/api/auth/account-tenants` resolves with more than one tenant — each story stubs `fetch` to simulate that response.",
      },
    },
  },
} satisfies Meta<typeof TenantSwitcher>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * The account only has access to one tenant (the storybook default), so the
 * switcher intentionally renders nothing.
 */
export const SingleTenantRendersNothing: Story = {
  beforeEach: () => mockAccountTenants(singleTenant),
};

/**
 * Multiple tenants under the same organization — the dropdown trigger shows
 * the current tenant name; no org-group headers are needed since there's
 * only one organization among the "other" tenants.
 */
export const MultipleTenantsSameOrganization: Story = {
  beforeEach: () => mockAccountTenants(sameOrgTenants),
};

/**
 * Tenants span two organizations — opening the dropdown shows an
 * organization-name header above each group.
 */
export const MultipleOrganizationsOpen: Story = {
  beforeEach: () => mockAccountTenants(multiOrgTenants),
  play: async ({ canvasElement }) => {
    const deadline = Date.now() + 2000;
    let trigger: HTMLButtonElement | null = null;
    while (Date.now() < deadline) {
      trigger = canvasElement.querySelector("button");
      if (trigger) break;
      await new Promise((resolve) => setTimeout(resolve, 50));
    }
    trigger?.click();
  },
};
