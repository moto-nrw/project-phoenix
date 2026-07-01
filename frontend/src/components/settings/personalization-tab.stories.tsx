import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ToastProvider } from "~/contexts/ToastContext";
import { PersonalizationTab } from "~/components/settings/personalization-tab";

const meta = {
  title: "settings/PersonalizationTab",
  component: PersonalizationTab,
  decorators: [
    (Story) => (
      <ToastProvider>
        <Story />
      </ToastProvider>
    ),
  ],
  parameters: {
    // The component fetches /api/settings/login-image on mount. In
    // Storybook this request fails (no backend), which the component
    // handles by keeping the current preview state and setting
    // isLoading to false — expected and harmless for demoing the layout.
    docs: { description: { component: "" } },
  },
} satisfies Meta<typeof PersonalizationTab>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default state of the personalization tab as embedded on the tenant
 * settings page.
 */
export const Default: Story = {};
