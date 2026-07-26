import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ToastProvider } from "~/contexts/ToastContext";
import { EnrollmentFormEditor } from "./enrollment-form-editor";

// EnrollmentFormEditor loads its data (schemas, phases, public legal texts)
// via effects on mount. There is no backend in Storybook, so the component
// renders its own loading state and then its error state once the requests
// fail — that is expected and exercised here rather than mocked away.
const meta: Meta<typeof EnrollmentFormEditor> = {
  title: "enrollment/EnrollmentFormEditor",
  component: EnrollmentFormEditor,
  decorators: [
    (Story) => (
      <ToastProvider>
        <Story />
      </ToastProvider>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof EnrollmentFormEditor>;

export const Default: Story = {};
