import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ProfileProvider } from "~/lib/profile-context";
import { TeacherShellProvider } from "~/lib/shell-auth-context";
import { LogoutModal } from "./logout-modal";

const meta = {
  title: "components/ui/LogoutModal",
  component: LogoutModal,
  decorators: [
    (Story) => (
      <ProfileProvider>
        <TeacherShellProvider>
          <Story />
        </TeacherShellProvider>
      </ProfileProvider>
    ),
  ],
  args: {
    isOpen: true,
    onClose: () => undefined,
  },
} satisfies Meta<typeof LogoutModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};
