import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { MFAEnrollmentScreen } from "~/components/auth/mfa-enrollment-screen";

const meta: Meta<typeof MFAEnrollmentScreen> = {
  title: "components/auth/MFAEnrollmentScreen",
  component: MFAEnrollmentScreen,
};

export default meta;
type Story = StoryObj<typeof MFAEnrollmentScreen>;

export const Default: Story = {
  args: {
    scope: "tenant",
    bearerToken: "storybook-enrollment-token",
    userEmail: "lehrer@storybook-schule.de",
    onComplete: () => {
      // no-op for the story
    },
  },
};
