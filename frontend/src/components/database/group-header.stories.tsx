import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";

import { Button } from "~/components/ui/button";

import { GroupHeader } from "./group-header";

const meta = {
  title: "database/GroupHeader",
  component: GroupHeader,
  args: {
    title: "Klasse 3a",
    count: 12,
    isOpen: true,
    onToggle: () => {
      // no-op for stories
    },
  },
} satisfies Meta<typeof GroupHeader>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Neutral: Story = {
  args: {
    variant: "neutral",
  },
};

export const Warning: Story = {
  args: {
    variant: "warning",
    title: "Fehlende Angaben",
    count: 3,
  },
};

export const Success: Story = {
  args: {
    variant: "success",
    title: "Vollständig",
    count: 8,
    countSuffix: "Kinder",
  },
};

export const Collapsed: Story = {
  args: {
    isOpen: false,
  },
};

export const WithBulkAction: Story = {
  args: {
    bulkAction: (
      <Button type="button" size="compact" variant="ghost">
        Aktion
      </Button>
    ),
  },
};

export const Interactive: Story = {
  render: (args) => {
    function InteractiveGroupHeader() {
      const [isOpen, setIsOpen] = useState(args.isOpen);
      return (
        <GroupHeader
          {...args}
          isOpen={isOpen}
          onToggle={() => {
            setIsOpen((prev) => !prev);
          }}
        />
      );
    }
    return <InteractiveGroupHeader />;
  },
};
