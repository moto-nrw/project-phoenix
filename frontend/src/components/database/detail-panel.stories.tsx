import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { DetailPanel, type DetailTab } from "./detail-panel";

const meta: Meta<typeof DetailPanel> = {
  title: "database/DetailPanel",
  component: DetailPanel,
};

export default meta;
type Story = StoryObj<typeof DetailPanel>;

const tabs: DetailTab[] = [
  { id: "overview", label: "Übersicht", content: <div>Übersichtsinhalt</div> },
  { id: "history", label: "Historie", content: <div>Historieninhalt</div> },
  {
    id: "disabled",
    label: "Gesperrt",
    content: <div>Nicht verfügbar</div>,
    disabled: true,
  },
];

function DetailPanelDemo() {
  const [activeTab, setActiveTab] = useState("overview");
  return (
    <div style={{ height: 400 }}>
      <DetailPanel
        header={<div className="p-4 font-semibold">Max Mustermann</div>}
        tabs={tabs}
        activeTab={activeTab}
        onTabChange={setActiveTab}
      />
    </div>
  );
}

export const Default: Story = {
  render: () => <DetailPanelDemo />,
};
