import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { PickupExceptionFormModal } from "./pickup-exception-form-modal";
import type { PickupException } from "@/lib/pickup-schedule-helpers";

const meta = {
  title: "students/PickupExceptionFormModal",
  component: PickupExceptionFormModal,
  parameters: {
    layout: "fullscreen",
  },
  args: {
    isOpen: true,
    onClose: fn(),
    onSubmit: fn(async () => {}),
    mode: "create",
  },
} satisfies Meta<typeof PickupExceptionFormModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Create: Story = {
  args: {
    mode: "create",
  },
};

export const CreateWithDefaultDate: Story = {
  args: {
    mode: "create",
    defaultDate: "2026-07-15",
  },
};

const editException: PickupException = {
  id: "1",
  studentId: "42",
  exceptionDate: "2026-07-10",
  pickupTime: "14:30",
  reason: "Arzttermin",
  source: "staff",
  createdBy: "1",
  createdAt: "2026-07-01T10:00:00Z",
  updatedAt: "2026-07-01T10:00:00Z",
};

export const Edit: Story = {
  args: {
    mode: "edit",
    initialData: editException,
  },
};

export const Closed: Story = {
  args: {
    isOpen: false,
    mode: "create",
  },
};
