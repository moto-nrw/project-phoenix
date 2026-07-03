import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";

import { MessageComposer } from "./message-composer";

const meta: Meta<typeof MessageComposer> = {
  title: "components/messaging/MessageComposer",
  component: MessageComposer,
};

export default meta;
type Story = StoryObj<typeof MessageComposer>;

function ControlledComposer(props: {
  readonly initialValue?: string;
  readonly sending?: boolean;
  readonly disabled?: boolean;
  readonly placeholder?: string;
}) {
  const [value, setValue] = useState(props.initialValue ?? "");
  return (
    <div className="max-w-xl p-4">
      <MessageComposer
        value={value}
        onChange={setValue}
        onSend={() => setValue("")}
        sending={props.sending ?? false}
        disabled={props.disabled ?? false}
        placeholder={props.placeholder}
      />
    </div>
  );
}

export const Empty: Story = {
  render: () => <ControlledComposer />,
};

export const WithText: Story = {
  render: () => <ControlledComposer initialValue="Hallo, wie geht es Ihnen?" />,
};

export const Sending: Story = {
  render: () => (
    <ControlledComposer initialValue="Nachricht wird gesendet..." sending />
  ),
};

export const Disabled: Story = {
  render: () => (
    <ControlledComposer initialValue="Dieser Chat ist deaktiviert" disabled />
  ),
};
