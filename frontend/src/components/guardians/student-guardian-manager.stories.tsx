import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ToastProvider } from "~/contexts/ToastContext";
import StudentGuardianManager from "~/components/guardians/student-guardian-manager";

const meta = {
  title: "guardians/StudentGuardianManager",
  component: StudentGuardianManager,
  decorators: [
    (Story) => (
      <ToastProvider>
        <div className="max-w-2xl">
          <Story />
        </div>
      </ToastProvider>
    ),
  ],
} satisfies Meta<typeof StudentGuardianManager>;

export default meta;

type Story = StoryObj<typeof meta>;

// The component fetches the student's guardians on mount via the real
// guardian-api client. In Storybook there is no backend to answer that
// request, so it exercises the component's own error-handling path
// (loading -> failed fetch -> inline error banner), which is a real,
// user-visible state of this component.
export const Default: Story = {
  args: {
    studentId: "1",
  },
};

export const ReadOnly: Story = {
  args: {
    studentId: "1",
    readOnly: true,
  },
};
