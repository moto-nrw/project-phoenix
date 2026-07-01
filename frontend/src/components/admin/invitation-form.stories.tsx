import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ToastProvider } from "~/contexts/ToastContext";
import { InvitationForm } from "./invitation-form";

const meta = {
  title: "components/admin/InvitationForm",
  component: InvitationForm,
  decorators: [
    (Story) => (
      <ToastProvider>
        <Story />
      </ToastProvider>
    ),
  ],
  args: {
    onCreated: () => undefined,
    existingPositions: [],
  },
} satisfies Meta<typeof InvitationForm>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const WithExistingPositions: Story = {
  args: {
    existingPositions: ["Pädagogische Fachkraft", "OGS-Büro"],
  },
};
