import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { DesktopOnlyNotice } from "./desktop-only-notice";

const meta = {
  title: "components/ui/DesktopOnlyNotice",
  component: DesktopOnlyNotice,
} satisfies Meta<typeof DesktopOnlyNotice>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const CustomTitle: Story = {
  args: {
    title: "Diese Ansicht ist nur am Computer verfügbar",
  },
};

export const CustomDescription: Story = {
  args: {
    title: "Diese Ansicht ist nur am Computer verfügbar",
    description:
      "Der Formularbaukasten braucht zum Sortieren der Felder einen großen Bildschirm.",
  },
};
