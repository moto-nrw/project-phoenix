import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { SearchBar } from "./SearchBar";

function ControlledSearchBar(props: React.ComponentProps<typeof SearchBar>) {
  const [value, setValue] = useState(props.value);
  return <SearchBar {...props} value={value} onChange={setValue} />;
}

const meta: Meta<typeof SearchBar> = {
  title: "components/ui/page-header/SearchBar",
  component: SearchBar,
  render: (args) => <ControlledSearchBar {...args} />,
  args: {
    value: "",
    onChange: () => undefined,
  },
};

export default meta;
type Story = StoryObj<typeof SearchBar>;

export const Default: Story = {};

export const WithValue: Story = {
  args: {
    value: "Max Mustermann",
  },
};

export const Small: Story = {
  args: {
    size: "sm",
  },
};

export const Large: Story = {
  args: {
    size: "lg",
    value: "Suchbegriff",
  },
};
