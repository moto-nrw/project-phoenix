import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";

import { TextareaField } from "./textarea-field";

const meta = {
  title: "components/settings/fields/TextareaField",
  component: TextareaField,
  parameters: {
    layout: "padded",
  },
} satisfies Meta<typeof TextareaField>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Empty: Story = {
  args: {
    value: "",
    onChange: () => {
      // no-op for story fixture
    },
  },
};

export const WithContent: Story = {
  args: {
    value:
      "# Allgemeine Geschäftsbedingungen\n\nDies ist ein Beispieltext im Markdown-Format, der von der öffentlichen Anmeldeseite gerendert wird.",
    onChange: () => {
      // no-op for story fixture
    },
  },
};

export const Disabled: Story = {
  args: {
    value: "Dieser Text kann derzeit nicht bearbeitet werden.",
    onChange: () => {
      // no-op for story fixture
    },
    disabled: true,
  },
};

export const Interactive: Story = {
  args: {
    value: "Bearbeite diesen Text, um das Verhalten zu testen.",
    onChange: () => {
      // no-op for story fixture; render() below manages its own state
    },
  },
  render: function Render() {
    const [value, setValue] = useState(
      "Bearbeite diesen Text, um das Verhalten zu testen.",
    );
    return <TextareaField value={value} onChange={setValue} />;
  },
};
