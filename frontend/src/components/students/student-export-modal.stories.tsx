import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { StudentExportModal } from "./student-export-modal";
import { ToastProvider } from "~/contexts/ToastContext";
import type { StudentExportFilters } from "~/lib/student-export-api";

const emptyFilters: StudentExportFilters = {};

const meta = {
  title: "students/StudentExportModal",
  component: StudentExportModal,
  decorators: [
    (Story) => (
      <ToastProvider>
        <Story />
      </ToastProvider>
    ),
  ],
  parameters: {
    layout: "fullscreen",
  },
  args: {
    isOpen: true,
    filters: emptyFilters,
    resultCount: 42,
    onClose: () => {
      // no-op for story
    },
  },
} satisfies Meta<typeof StudentExportModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Open: Story = {};

export const WithClassFilter: Story = {
  args: {
    filters: { school_class: "3a" },
    resultCount: 18,
  },
};

export const Closed: Story = {
  args: {
    isOpen: false,
  },
};
