import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { ModalProvider, useModal } from "./modal-context";

function ModalStatusDemo() {
  const { isModalOpen, openModal, closeModal } = useModal();

  return (
    <div className="flex flex-col gap-3 rounded-2xl border border-gray-200 bg-white p-6 shadow-sm">
      <p>
        Modal offen: <strong>{isModalOpen ? "ja" : "nein"}</strong>
      </p>
      <div className="flex gap-2">
        <button
          type="button"
          className="rounded-md bg-gray-900 px-4 py-2 text-sm text-white"
          onClick={openModal}
        >
          Öffnen
        </button>
        <button
          type="button"
          className="rounded-md border border-gray-200 px-4 py-2 text-sm"
          onClick={closeModal}
        >
          Schließen
        </button>
      </div>
    </div>
  );
}

const meta = {
  title: "dashboard/ModalContext",
  component: ModalStatusDemo,
} satisfies Meta<typeof ModalStatusDemo>;

export default meta;

type Story = StoryObj<typeof meta>;

export const WithinProvider: Story = {
  render: () => (
    <ModalProvider>
      <ModalStatusDemo />
    </ModalProvider>
  ),
};

export const WithoutProvider: Story = {
  render: () => <ModalStatusDemo />,
};
