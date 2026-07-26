import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { GuidePdfButton } from "./guide-pdf-button";

const meta = {
  title: "help/GuidePdfButton",
  component: GuidePdfButton,
} satisfies Meta<typeof GuidePdfButton>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    href: "/help/pdfs/die-app-im-alltag.pdf",
    download: "Die App im Alltag.pdf",
  },
};
