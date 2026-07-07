import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";

import { PasswordToggleButton } from "./password-toggle-button";

const meta: Meta<typeof PasswordToggleButton> = {
  title: "shared/PasswordToggleButton",
  component: PasswordToggleButton,
};

export default meta;
type Story = StoryObj<typeof PasswordToggleButton>;

export const Hidden: Story = {
  args: {
    showPassword: false,
    onToggle: () => undefined,
  },
};

export const Shown: Story = {
  args: {
    showPassword: true,
    onToggle: () => undefined,
  },
};

export const CustomLabels: Story = {
  args: {
    showPassword: false,
    onToggle: () => undefined,
    showLabel: "Show password",
    hideLabel: "Hide password",
  },
};

function InteractiveDemo() {
  const [showPassword, setShowPassword] = useState(false);
  return (
    <div className="relative w-40">
      <PasswordToggleButton
        showPassword={showPassword}
        onToggle={() => setShowPassword((prev) => !prev)}
      />
    </div>
  );
}

export const Interactive: Story = {
  render: () => <InteractiveDemo />,
};
