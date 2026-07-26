import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { ForbiddenPage } from "./forbidden-page";

const meta = {
  title: "components/ui/ForbiddenPage",
  component: ForbiddenPage,
} satisfies Meta<typeof ForbiddenPage>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const CustomMessage: Story = {
  args: {
    title: "Zugriff verweigert",
    message: "Diese Seite ist nur für Administratoren zugänglich.",
  },
};
