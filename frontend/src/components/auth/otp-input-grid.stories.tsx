import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { OTPInputGrid } from "./otp-input-grid";

const meta = {
  title: "components/auth/OTPInputGrid",
  component: OTPInputGrid,
  args: {
    length: 6,
    onComplete: () => {
      // no-op for stories
    },
  },
} satisfies Meta<typeof OTPInputGrid>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Disabled: Story = {
  args: {
    disabled: true,
  },
};

export const FourDigits: Story = {
  args: {
    length: 4,
  },
};
