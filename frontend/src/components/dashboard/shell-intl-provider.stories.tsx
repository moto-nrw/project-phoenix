import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { ShellIntlProvider } from "./shell-intl-provider";

const meta = {
  title: "components/dashboard/ShellIntlProvider",
  component: ShellIntlProvider,
} satisfies Meta<typeof ShellIntlProvider>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    children: <div>Shell navigation content</div>,
  },
};
