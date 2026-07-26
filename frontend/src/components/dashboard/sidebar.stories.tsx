import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { SessionProvider } from "next-auth/react";
import { ProfileProvider } from "~/lib/profile-context";
import {
  TeacherShellProvider,
  OperatorShellProvider,
  ParentShellProvider,
} from "~/lib/shell-auth-context";
import {
  mockSessionData,
  mockTeacherSessionData,
} from "~storybook/fixtures/session";
import { Sidebar } from "./sidebar";

const meta = {
  title: "components/dashboard/Sidebar",
  component: Sidebar,
  parameters: {
    // The sidebar is a full-height column; give it room to render.
    layout: "fullscreen",
  },
} satisfies Meta<typeof Sidebar>;

export default meta;

type Story = StoryObj<typeof meta>;

export const StaffMode: Story = {
  decorators: [
    (Story) => (
      <SessionProvider session={mockTeacherSessionData()}>
        <ProfileProvider>
          <TeacherShellProvider>
            <Story />
          </TeacherShellProvider>
        </ProfileProvider>
      </SessionProvider>
    ),
  ],
};

export const AdminMode: Story = {
  decorators: [
    (Story) => (
      <SessionProvider
        session={mockSessionData({ user: { roles: ["admin"] } })}
      >
        <ProfileProvider>
          <TeacherShellProvider>
            <Story />
          </TeacherShellProvider>
        </ProfileProvider>
      </SessionProvider>
    ),
  ],
};

export const OperatorMode: Story = {
  decorators: [
    (Story) => (
      <SessionProvider
        session={mockSessionData({
          user: { roles: ["operator"], permissions: [], scope: "platform" },
        })}
      >
        <OperatorShellProvider>
          <Story />
        </OperatorShellProvider>
      </SessionProvider>
    ),
  ],
};

export const ParentMode: Story = {
  decorators: [
    (Story) => (
      <SessionProvider
        session={mockSessionData({
          user: { roles: ["guardian"], permissions: [], scope: "parent" },
        })}
      >
        <ParentShellProvider>
          <Story />
        </ParentShellProvider>
      </SessionProvider>
    ),
  ],
};
