import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { ShellNavIntlProvider } from "./shell-nav-intl-provider";

const meta = {
  title: "components/dashboard/ShellNavIntlProvider",
  component: ShellNavIntlProvider,
} satisfies Meta<typeof ShellNavIntlProvider>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    children: <div>Shell navigation content</div>,
  },
};
