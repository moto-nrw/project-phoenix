import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { StudentCreateModal } from "./student-create-modal";

const meta = {
  title: "components/students/StudentCreateModal",
  component: StudentCreateModal,
  parameters: {
    layout: "fullscreen",
  },
  args: {
    isOpen: true,
    onClose: () => undefined,
    onCreate: async () => undefined,
  },
} satisfies Meta<typeof StudentCreateModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Open: Story = {};

export const Closed: Story = {
  args: {
    isOpen: false,
  },
};

export const WithGroups: Story = {
  args: {
    groups: [
      { value: "1", label: "Gruppe Sonne" },
      { value: "2", label: "Gruppe Mond" },
    ],
  },
};
