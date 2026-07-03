import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { EnrollmentEditPage } from "~/components/enrollment/enrollment-edit-page";

const meta: Meta<typeof EnrollmentEditPage> = {
  title: "components/enrollment/EnrollmentEditPage",
  component: EnrollmentEditPage,
};

export default meta;
type Story = StoryObj<typeof EnrollmentEditPage>;

/**
 * The component fetches its bootstrap data client-side via
 * `fetchEnrollmentEditBootstrap(token)`. There is no backend in Storybook,
 * so the fetch fails and the component renders its own error state
 * (with a "back to status" link) instead of crashing.
 */
export const LoadError: Story = {
  args: {
    params: Promise.resolve({ token: "storybook-token" }),
  },
};
