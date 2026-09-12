import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { DatabaseListItem } from "./database-list-item";
import { DatabaseListLayout } from "./database-list-layout";

const meta: Meta<typeof DatabaseListLayout> = {
  title: "components/database/DatabaseListLayout",
  component: DatabaseListLayout,
};

export default meta;

type Story = StoryObj<typeof DatabaseListLayout>;

export const Default: Story = {
  render: () => (
    <DatabaseListLayout>
      <div className="flex-1 overflow-auto">
        <DatabaseListItem
          title="Mia Fischer"
          subtitle="Klasse 3a · Gruppe Blau"
          isSelected={false}
          href="/students/1"
        />
        <DatabaseListItem
          title="Ben Weber"
          subtitle="Klasse 2b · Gruppe Rot"
          isSelected={false}
          href="/students/2"
        />
      </div>
    </DatabaseListLayout>
  ),
};
