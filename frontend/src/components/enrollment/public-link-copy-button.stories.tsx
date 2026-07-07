import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { PublicLinkCopyButton } from "./public-link-copy-button";

const meta = {
  title: "components/enrollment/PublicLinkCopyButton",
  component: PublicLinkCopyButton,
} satisfies Meta<typeof PublicLinkCopyButton>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    url: "https://storybook.moto-app.de/enroll/1",
    componentId: "public-link-copy-button-default",
  },
};

export const CustomLabel: Story = {
  args: {
    url: "https://storybook.moto-app.de/enroll/1",
    componentId: "public-link-copy-button-custom-label",
    label: "Link für Eltern kopieren",
  },
};
