import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ToastProvider } from "~/contexts/ToastContext";
import { PhasesEditor } from "./phases-editor";

// PhasesEditor loads its data (phases, form schemas) via effects on mount.
// There is no backend in Storybook, so the component renders its own
// loading state and then its error/empty state once the requests fail —
// that is expected and exercised here rather than mocked away.
const meta: Meta<typeof PhasesEditor> = {
  title: "enrollment/PhasesEditor",
  component: PhasesEditor,
  decorators: [
    (Story) => (
      <ToastProvider>
        <Story />
      </ToastProvider>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof PhasesEditor>;

export const Default: Story = {};
