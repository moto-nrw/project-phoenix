import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { Button } from "./button";

const meta = {
  title: "components/ui/Button",
  component: Button,
  args: {
    children: "Button",
  },
} satisfies Meta<typeof Button>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Primary: Story = {
  args: {
    variant: "primary",
  },
};

export const Secondary: Story = {
  args: {
    variant: "secondary",
  },
};

export const Outline: Story = {
  args: {
    variant: "outline",
  },
};

export const OutlineDanger: Story = {
  args: {
    variant: "outline_danger",
  },
};

export const Danger: Story = {
  args: {
    variant: "danger",
  },
};

export const Success: Story = {
  args: {
    variant: "success",
  },
};

export const Ghost: Story = {
  args: {
    variant: "ghost",
  },
};

export const Loading: Story = {
  args: {
    isLoading: true,
    loadingText: "Lädt...",
  },
};

export const Disabled: Story = {
  args: {
    disabled: true,
  },
};

export const Sizes: Story = {
  render: () => (
    <div className="flex flex-wrap items-center gap-3">
      <Button size="sm">sm</Button>
      <Button size="md">md</Button>
      <Button size="base">base</Button>
      <Button size="lg">lg</Button>
      <Button size="xl">xl</Button>
      <Button size="compact">compact</Button>
      <Button size="icon">i</Button>
    </div>
  ),
};
