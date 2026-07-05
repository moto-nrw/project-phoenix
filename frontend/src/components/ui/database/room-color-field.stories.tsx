import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { RoomColorField } from "./room-color-field";

/**
 * `RoomColorField` is a controlled component (`value` + `onChange`), so each
 * story wraps it in a tiny stateful shell to make the color picker and the
 * "Zurücksetzen" button interactive in the Storybook canvas.
 */
function ControlledRoomColorField(
  props: Readonly<{
    initialValue: unknown;
    label: string;
    required?: boolean;
  }>,
) {
  const { initialValue, label, required } = props;
  const [value, setValue] = useState<unknown>(initialValue);

  return (
    <RoomColorField
      value={value}
      onChange={setValue}
      label={label}
      required={required}
    />
  );
}

const meta = {
  title: "ui/database/RoomColorField",
  component: RoomColorField,
  args: {
    value: null,
    onChange: () => undefined,
    label: "Badge-Farbe",
  },
  parameters: {
    docs: {
      description: {
        component:
          "Color picker field for a room's badge color in the Database Rooms edit form. Falls back to the OTHER_ROOM blue preview when unset.",
      },
    },
  },
} satisfies Meta<typeof RoomColorField>;

export default meta;
type Story = StoryObj<typeof meta>;

/** No color set — shows the OTHER_ROOM blue fallback preview and "Standard" label. */
export const Unset: Story = {
  render: (args) => (
    <ControlledRoomColorField initialValue={null} label={args.label} />
  ),
};

/** A custom hex color is already set — shows the hex value and the reset button. */
export const CustomColorSet: Story = {
  render: (args) => (
    <ControlledRoomColorField initialValue="#F78C10" label={args.label} />
  ),
};

/** Required field variant — label carries the "*" suffix. */
export const Required: Story = {
  render: (args) => (
    <ControlledRoomColorField initialValue={null} label={args.label} required />
  ),
};
