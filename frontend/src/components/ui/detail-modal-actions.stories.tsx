import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";

import { DetailModalActions } from "~/components/ui/detail-modal-actions";

const meta = {
  title: "components/ui/DetailModalActions",
  component: DetailModalActions,
  parameters: {
    layout: "padded",
  },
  args: {
    onEdit: fn(),
    onDelete: fn(),
    entityName: "Gruppe Sonnenschein",
    entityType: "Gruppe",
  },
} satisfies Meta<typeof DetailModalActions>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const GermanArticleDas: Story = {
  args: {
    entityName: "Tablet 3",
    entityType: "Gerät",
  },
};

export const HideDelete: Story = {
  args: {
    hideDelete: true,
  },
};

export const CustomConfirmationContent: Story = {
  args: {
    confirmationContent: (
      <p className="text-sm text-gray-700">
        Diese Aktivität hat noch aktive Anmeldungen. Möchten Sie sie trotzdem
        löschen?
      </p>
    ),
  },
};

export const ExternalDeleteHandler: Story = {
  args: {
    onDeleteClick: fn(),
  },
};
