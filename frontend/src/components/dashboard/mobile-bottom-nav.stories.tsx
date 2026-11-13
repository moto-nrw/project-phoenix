import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, screen, userEvent, within } from "storybook/test";
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

/** Das geöffnete „Mehr“-Menü mit den Gruppen der Seitenleiste. */
export const StaffModeMoreOpened: Story = {
  decorators: StaffMode.decorators,
  play: async ({ canvasElement }) => {
    // Ab 1024 px blendet CSS die Leiste aus; dort gibt es nichts zu öffnen.
    const more = within(canvasElement).queryByRole("button", { name: "Mehr" });
    if (!more) return;
    await userEvent.click(more);
    await expect(await screen.findByRole("dialog")).toBeVisible();
  },
};

/** Eine Seite hinter „Mehr“ ist offen: die Fläche liegt unter „Mehr“. */
export const StaffModeActiveBehindMore: Story = {
  decorators: StaffMode.decorators,
  parameters: {
    nextjs: {
      appDirectory: true,
      navigation: { pathname: "/calendar", query: {} },
    },
  },
};
