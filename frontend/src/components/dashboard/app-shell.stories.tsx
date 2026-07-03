import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { BreadcrumbProvider } from "~/lib/breadcrumb-context";
import { ProfileProvider } from "~/lib/profile-context";
import { TeacherShellProvider } from "~/lib/shell-auth-context";

import { AppShell } from "./app-shell";

const meta = {
  title: "components/dashboard/AppShell",
  component: AppShell,
  parameters: {
    layout: "fullscreen",
  },
  decorators: [
    (Story) => (
      <BreadcrumbProvider>
        <ProfileProvider>
          <TeacherShellProvider>
            <Story />
          </TeacherShellProvider>
        </ProfileProvider>
      </BreadcrumbProvider>
    ),
  ],
} satisfies Meta<typeof AppShell>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    children: (
      <div className="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm">
        Seiteninhalt
      </div>
    ),
  },
};
