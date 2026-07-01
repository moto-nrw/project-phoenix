import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { LanguageSwitcher } from "~/components/parent/language-switcher";

const meta: Meta<typeof LanguageSwitcher> = {
  title: "components/parent/LanguageSwitcher",
  component: LanguageSwitcher,
};

export default meta;
type Story = StoryObj<typeof LanguageSwitcher>;

export const Default: Story = {};

export const Compact: Story = {
  args: {
    compact: true,
  },
};
