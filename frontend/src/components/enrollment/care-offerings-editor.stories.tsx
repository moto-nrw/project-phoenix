import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ToastProvider } from "~/contexts/ToastContext";
import { CareOfferingsEditor } from "./care-offerings-editor";

// CareOfferingsEditor loads phases, offerings, Regeltermine, and planning
// periods on mount. Storybook has no backend, so this story intentionally
// exercises the component's loading and error states without mocks.
const meta: Meta<typeof CareOfferingsEditor> = {
  title: "enrollment/CareOfferingsEditor",
  component: CareOfferingsEditor,
  decorators: [
    (Story) => (
      <ToastProvider>
        <Story />
      </ToastProvider>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof CareOfferingsEditor>;

export const Default: Story = {};
