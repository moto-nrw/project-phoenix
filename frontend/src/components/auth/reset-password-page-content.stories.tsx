import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ResetPasswordPageContent } from "~/components/auth/reset-password-page-content";

const meta: Meta<typeof ResetPasswordPageContent> = {
  title: "components/auth/ResetPasswordPageContent",
  component: ResetPasswordPageContent,
  parameters: {
    nextjs: {
      appDirectory: true,
      navigation: {
        pathname: "/reset-password",
        query: {},
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof ResetPasswordPageContent>;

export const MissingToken: Story = {};

export const WithToken: Story = {
  parameters: {
    nextjs: {
      appDirectory: true,
      navigation: {
        pathname: "/reset-password",
        query: { token: "fake-reset-token" },
      },
    },
  },
};
