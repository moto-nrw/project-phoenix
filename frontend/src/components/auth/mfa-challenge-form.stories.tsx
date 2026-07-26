import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { MFAChallengeForm } from "~/components/auth/mfa-challenge-form";

const meta: Meta<typeof MFAChallengeForm> = {
  title: "components/auth/MFAChallengeForm",
  component: MFAChallengeForm,
};

export default meta;
type Story = StoryObj<typeof MFAChallengeForm>;

export const Default: Story = {
  args: {
    scope: "tenant",
    challengeToken: "fake-challenge-token",
    maskedEmail: "j***@example.com",
    onSuccess: () => {},
  },
};

export const WithCancelAndCustomTrustedDevice: Story = {
  args: {
    scope: "tenant",
    challengeToken: "fake-challenge-token",
    maskedEmail: "j***@example.com",
    resendCooldownSeconds: 30,
    trustedDeviceEnabled: true,
    trustedDeviceDays: 14,
    onSuccess: () => {},
    onCancel: () => {},
  },
};

export const TrustedDeviceDisabled: Story = {
  args: {
    scope: "tenant",
    challengeToken: "fake-challenge-token",
    maskedEmail: "j***@example.com",
    trustedDeviceEnabled: false,
    onSuccess: () => {},
    onCancel: () => {},
  },
};
