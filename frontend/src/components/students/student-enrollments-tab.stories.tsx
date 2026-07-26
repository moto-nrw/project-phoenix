import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ToastProvider } from "~/contexts/ToastContext";
import { StudentEnrollmentsTab } from "./student-enrollments-tab";

const meta: Meta<typeof StudentEnrollmentsTab> = {
  title: "components/students/StudentEnrollmentsTab",
  component: StudentEnrollmentsTab,
  decorators: [
    (Story) => (
      <ToastProvider>
        <Story />
      </ToastProvider>
    ),
  ],
};

export default meta;

type Story = StoryObj<typeof StudentEnrollmentsTab>;

export const Default: Story = {
  args: {
    studentId: "1",
  },
};
