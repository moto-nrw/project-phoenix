import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ToastProvider } from "~/contexts/ToastContext";
import { TransitStudentsSection } from "~/components/rooms/transit-students-section";

const meta = {
  title: "rooms/TransitStudentsSection",
  component: TransitStudentsSection,
  decorators: [
    (Story) => (
      <ToastProvider>
        <Story />
      </ToastProvider>
    ),
  ],
  parameters: {
    // The component fetches students (locationState=transit) and active
    // groups via SWR on mount. In Storybook there is no backend, so these
    // requests fail and the section renders its error state ("Die
    // Unterwegs-Daten konnten nicht geladen werden.") — expected and
    // harmless for demoing the layout/chrome of the section.
    docs: { description: { component: "" } },
  },
} satisfies Meta<typeof TransitStudentsSection>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default state. No backend is available in Storybook, so this renders
 * the section's error state.
 */
export const Default: Story = {};
