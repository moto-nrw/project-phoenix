import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { GuardianInvitationAcceptForm } from "~/components/auth/guardian-invitation-accept-form";
import type { GuardianInvitationValidation } from "~/lib/guardian-invitation-api";

const baseInvitation: GuardianInvitationValidation = {
  email: "erika.musterfrau@example.com",
  firstName: "Erika",
  lastName: "Musterfrau",
  expiresAt: "2026-12-31T23:59:59Z",
  schoolName: "OGS Musterschule",
  tenantSlug: "musterschule",
};

const meta: Meta<typeof GuardianInvitationAcceptForm> = {
  title: "components/auth/GuardianInvitationAcceptForm",
  component: GuardianInvitationAcceptForm,
};

export default meta;
type Story = StoryObj<typeof GuardianInvitationAcceptForm>;

export const WithFullName: Story = {
  args: {
    token: "fake-invitation-token",
    invitation: baseInvitation,
  },
};

export const EmailOnly: Story = {
  args: {
    token: "fake-invitation-token",
    invitation: {
      email: "erika.musterfrau@example.com",
      expiresAt: "2026-12-31T23:59:59Z",
    },
  },
};
