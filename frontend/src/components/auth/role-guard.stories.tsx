import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { SessionProvider } from "next-auth/react";
import type { Session } from "next-auth";
import { RoleGuard } from "./role-guard";

function buildSession(roles: readonly string[]): Session {
  return {
    user: {
      id: "1",
      name: "Test User",
      email: "test@test.com",
      token: "test-token",
      roles: [...roles],
    },
    expires: "2099-01-01",
  };
}

const meta: Meta<typeof RoleGuard> = {
  title: "components/auth/RoleGuard",
  component: RoleGuard,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof RoleGuard>;

export const AdminOnlyAllowed: Story = {
  render: (args) => (
    <SessionProvider session={buildSession(["admin"])}>
      <RoleGuard {...args} />
    </SessionProvider>
  ),
  args: {
    variant: "adminOnly",
    children: <div className="p-6">Geschützter Admin-Inhalt</div>,
  },
};

export const AdminOnlyForbidden: Story = {
  render: (args) => (
    <SessionProvider session={buildSession(["user"])}>
      <RoleGuard {...args} />
    </SessionProvider>
  ),
  args: {
    variant: "adminOnly",
    children: <div className="p-6">Geschützter Admin-Inhalt</div>,
    message: "Dieser Bereich ist nur für Administratoren zugänglich.",
  },
};

export const StaffOnlyAllowed: Story = {
  render: (args) => (
    <SessionProvider session={buildSession(["teacher"])}>
      <RoleGuard {...args} />
    </SessionProvider>
  ),
  args: {
    variant: "staffOnly",
    children: <div className="p-6">Geschützter Mitarbeiter-Inhalt</div>,
  },
};
