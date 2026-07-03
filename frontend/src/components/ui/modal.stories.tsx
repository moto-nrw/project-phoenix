import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Modal, ConfirmationModal } from "./modal";

const meta = {
  title: "components/ui/Modal",
  component: Modal,
  args: {
    isOpen: true,
    onClose: () => undefined,
    title: "Modal-Titel",
    children: (
      <p>
        Dies ist der Inhalt des Modals. Hier können beliebige Inhalte angezeigt
        werden.
      </p>
    ),
  },
} satisfies Meta<typeof Modal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const WithFooter: Story = {
  args: {
    footer: (
      <button
        type="button"
        className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white"
      >
        Speichern
      </button>
    ),
  },
};

export const WithoutTitle: Story = {
  args: {
    title: "",
  },
};

export const WideContent: Story = {
  args: {
    widthClass: "mx-4 w-[calc(100%-2rem)] max-w-4xl",
  },
};

export const ConfirmationDefault: StoryObj<typeof ConfirmationModal> = {
  render: (args) => <ConfirmationModal {...args} />,
  args: {
    isOpen: true,
    onClose: () => undefined,
    onConfirm: () => undefined,
    title: "Wirklich löschen?",
    children: <p>Diese Aktion kann nicht rückgängig gemacht werden.</p>,
  },
};

export const ConfirmationLoading: StoryObj<typeof ConfirmationModal> = {
  render: (args) => <ConfirmationModal {...args} />,
  args: {
    isOpen: true,
    onClose: () => undefined,
    onConfirm: () => undefined,
    title: "Wirklich löschen?",
    children: <p>Diese Aktion kann nicht rückgängig gemacht werden.</p>,
    isConfirmLoading: true,
  },
};
