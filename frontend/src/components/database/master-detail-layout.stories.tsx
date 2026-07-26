import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { MasterDetailLayout } from "./master-detail-layout";

const meta: Meta<typeof MasterDetailLayout> = {
  title: "components/database/MasterDetailLayout",
  component: MasterDetailLayout,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof MasterDetailLayout>;

const ListContent = () => (
  <ul className="divide-y divide-gray-100">
    {["Anna Müller", "Ben Schmidt", "Clara Weber"].map((name) => (
      <li key={name} className="p-4 text-sm text-gray-900">
        {name}
      </li>
    ))}
  </ul>
);

const DetailContent = () => (
  <div className="p-6">
    <h2 className="text-lg font-semibold text-gray-900">Anna Müller</h2>
    <p className="mt-2 text-sm text-gray-600">
      Details des ausgewählten Kindes.
    </p>
  </div>
);

export const Selected: Story = {
  args: {
    list: <ListContent />,
    detail: <DetailContent />,
    selectedId: "1",
    onDeselect: () => {},
  },
};

export const Unselected: Story = {
  args: {
    list: <ListContent />,
    detail: <DetailContent />,
    selectedId: null,
    onDeselect: () => {},
  },
};

export const ExpandWhenUnselected: Story = {
  args: {
    list: <ListContent />,
    detail: <DetailContent />,
    selectedId: null,
    onDeselect: () => {},
    unselectedBehavior: "expand",
  },
};

export const Interactive: Story = {
  render: () => {
    function InteractiveDemo() {
      const [selectedId, setSelectedId] = useState<string | null>("1");
      return (
        <MasterDetailLayout
          list={<ListContent />}
          detail={<DetailContent />}
          selectedId={selectedId}
          onDeselect={() => setSelectedId(null)}
        />
      );
    }
    return <InteractiveDemo />;
  },
};
