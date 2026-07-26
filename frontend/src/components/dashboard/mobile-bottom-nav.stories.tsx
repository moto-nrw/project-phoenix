import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { SessionProvider } from "next-auth/react";
import { ProfileProvider } from "~/lib/profile-context";
import {
  TeacherShellProvider,
  OperatorShellProvider,
} from "~/lib/shell-auth-context";
import {
  mockSessionData,
  mockTeacherSessionData,
} from "~storybook/fixtures/session";
import { MobileBottomNav } from "./mobile-bottom-nav";

const meta = {
  title: "components/dashboard/MobileBottomNav",
  component: MobileBottomNav,
  parameters: {
    // The nav is fixed to the viewport bottom, so give it room to render.
    layout: "fullscreen",
  },
} satisfies Meta<typeof MobileBottomNav>;

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
        session={{
          user: { ...mockSessionData().user, roles: ["admin"] },
          expires: mockSessionData().expires,
        }}
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
