import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { InvitationAcceptForm } from "~/components/auth/invitation-accept-form";
import type { InvitationValidation } from "~/lib/invitation-helpers";

const baseInvitation: InvitationValidation = {
  email: "neue.person@schule.de",
  roleName: "teacher",
  firstName: "Max",
  lastName: "Mustermann",
  position: null,
  expiresAt: "2026-08-01T12:00:00Z",
};

const meta: Meta<typeof InvitationAcceptForm> = {
  title: "components/auth/InvitationAcceptForm",
  component: InvitationAcceptForm,
};

export default meta;
type Story = StoryObj<typeof InvitationAcceptForm>;

export const Default: Story = {
  args: {
    token: "fake-invitation-token",
    invitation: baseInvitation,
  },
};

export const WithPosition: Story = {
  args: {
    token: "fake-invitation-token",
    invitation: {
      ...baseInvitation,
      position: "Gruppenleitung 3b",
    },
  },
};

export const WithoutPrefilledName: Story = {
  args: {
    token: "fake-invitation-token",
    invitation: {
      ...baseInvitation,
      firstName: null,
      lastName: null,
    },
  },
};
