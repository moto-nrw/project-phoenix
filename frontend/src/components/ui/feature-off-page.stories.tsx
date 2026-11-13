import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { FeatureOffPage } from "./feature-off-page";

const meta = {
  title: "components/ui/FeatureOffPage",
  component: FeatureOffPage,
  args: {
    title: "NFC-Armbänder",
    description: "Hier verwalten Sie die NFC-Armbänder der Kinder.",
  },
} satisfies Meta<typeof FeatureOffPage>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const WhoCanEnable: Story = {
  args: {
    whoCanEnable: "Die Leitung kann sie in den Einstellungen einschalten.",
  },
};
