import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { SupervisionProvider } from "~/lib/supervision-context";
import { SmartRedirect } from "./smart-redirect";

const meta: Meta<typeof SmartRedirect> = {
  title: "components/auth/SmartRedirect",
  component: SmartRedirect,
  decorators: [
    (Story) => (
      <SupervisionProvider>
        <Story />
      </SupervisionProvider>
    ),
  ],
};

export default meta;

type Story = StoryObj<typeof SmartRedirect>;

/**
 * SmartRedirect renders nothing — it is a side-effect-only component that
 * redirects authenticated users once supervision data is ready. This story
 * confirms it mounts without crashing given the global fake session,
 * NextIntl, and Tenant providers plus a local SupervisionProvider.
 */
export const Default: Story = {};

/**
 * With an `onRedirect` callback supplied, the component calls it instead of
 * navigating directly — useful for hosts that want to control navigation
 * (still renders nothing visually).
 */
export const WithOnRedirectCallback: Story = {
  args: {
    onRedirect: () => {
      // no-op: demonstrates that redirect navigation is delegated to the
      // caller instead of using the router directly.
    },
  },
};
