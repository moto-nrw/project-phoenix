import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ToastProvider } from "~/contexts/ToastContext";
import { CareOfferingsEditor } from "./care-offerings-editor";

// CareOfferingsEditor loads its data (phases, care offerings, timetable
// templates) via effects on mount. There is no backend in Storybook, so the
// component renders its own loading state and then its error/empty state
// once the requests fail — that is expected and exercised here rather than
// mocked away.
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
