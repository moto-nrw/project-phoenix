import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { InvitationPageContent } from "~/components/auth/invitation-page-content";

const meta: Meta<typeof InvitationPageContent> = {
  title: "components/auth/InvitationPageContent",
  component: InvitationPageContent,
};

export default meta;
type Story = StoryObj<typeof InvitationPageContent>;

export const NoToken: Story = {
  args: {
    token: null,
  },
};

export const WithToken: Story = {
  args: {
    token: "fake-invitation-token",
  },
};
