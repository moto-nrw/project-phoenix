import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { SessionProvider } from "next-auth/react";
import { TenantGuard } from "~/components/tenant/tenant-guard";

const meta = {
  title: "tenant/TenantGuard",
  component: TenantGuard,
  parameters: {
    // TenantGuard's effects call real next-auth functions (signOut, signIn)
    // as side effects. The rendered output is driven directly by session
    // state, not by effect completion, so this is safe to demo — but the
    // network calls will 404 in Storybook, which is expected and harmless.
    docs: { description: { component: "" } },
  },
} satisfies Meta<typeof TenantGuard>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default state: session has no tenantId, so no mismatch is detected and
 * children render normally (this mirrors the global Storybook session mock).
 */
export const Default: Story = {
  args: {
    children: (
      <div className="p-6 text-sm text-gray-700">Geschützter Seiteninhalt</div>
    ),
  },
};

/**
 * Session tenant differs from the URL tenant (global Storybook tenant has
 * tenantId 1) — the guard renders the "switching" placeholder while it
 * attempts an auto-switch.
 */
export const TenantMismatchSwitching: Story = {
  args: {
    children: (
      <div className="p-6 text-sm text-gray-700">Geschützter Seiteninhalt</div>
    ),
  },
  decorators: [
    (Story) => (
      <SessionProvider
        session={{
          user: {
            id: "1",
            roles: ["admin"],
            token: "test-token",
            refreshToken: "test-refresh-token",
            tenantId: 2,
            scope: "",
          },
          expires: "2099-01-01",
        }}
      >
        <Story />
      </SessionProvider>
    ),
  ],
};

/**
 * A platform-scoped (operator) session leaked onto a tenant subdomain — the
 * guard blocks rendering and signs the session out.
 */
export const OperatorSessionOnTenant: Story = {
  args: {
    children: (
      <div className="p-6 text-sm text-gray-700">Geschützter Seiteninhalt</div>
    ),
  },
  decorators: [
    (Story) => (
      <SessionProvider
        session={{
          user: {
            id: "1",
            roles: ["operator"],
            token: "test-token",
            refreshToken: "test-refresh-token",
            tenantId: 1,
            scope: "platform",
          },
          expires: "2099-01-01",
        }}
      >
        <Story />
      </SessionProvider>
    ),
  ],
};

/**
 * The JWT refresh callback stripped the token (refresh failed) while
 * Auth.js still reports "authenticated" — the guard shows a renewing
 * placeholder instead of rendering tenant UI in that limbo state.
 */
export const ExpiredSession: Story = {
  args: {
    children: (
      <div className="p-6 text-sm text-gray-700">Geschützter Seiteninhalt</div>
    ),
  },
  decorators: [
    (Story) => (
      <SessionProvider
        session={{
          user: {
            id: "1",
            roles: ["admin"],
            token: "",
            refreshToken: "",
            tenantId: 1,
            scope: "",
          },
          expires: "2099-01-01",
          error: "RefreshTokenExpired",
        }}
      >
        <Story />
      </SessionProvider>
    ),
  ],
};
