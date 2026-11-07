import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { RemovableChip } from "./removable-chip";

const meta = {
  title: "UI/RemovableChip",
  component: RemovableChip,
} satisfies Meta<typeof RemovableChip>;

export default meta;

export const Default: StoryObj = {
  render: () => (
    <div className="flex flex-wrap gap-2">
      <RemovableChip
        label="Klasse 1a"
        onRemove={() => undefined}
        removeLabel="Klasse 1a entfernen"
      />
      <RemovableChip
        label="Impfpass.pdf"
        detail="1,2 MB"
        onRemove={() => undefined}
        removeLabel="Impfpass.pdf entfernen"
      />
      <RemovableChip label="Nur lesen" />
    </div>
  ),
};
