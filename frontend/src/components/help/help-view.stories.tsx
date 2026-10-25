import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { HelpView } from "./help-view";

const meta = { title: "help/LoginGuides", component: HelpView } satisfies Meta<
  typeof HelpView
>;
export default meta;
type Story = StoryObj<typeof meta>;

export const StaffLogin: Story = {
  parameters: {
    nextjs: {
      appDirectory: true,
      navigation: {
        pathname: "/help/bei-moto-anmelden",
        query: { role: "lead" },
        segments: [["topic", ["bei-moto-anmelden"]]],
      },
    },
  },
};
export const SchoolLogin: Story = {
  parameters: {
    nextjs: {
      appDirectory: true,
      navigation: {
        pathname: "/help/bei-moto-schule-anmelden",
        query: { role: "teacher" },
        segments: [["topic", ["bei-moto-schule-anmelden"]]],
      },
    },
  },
};
