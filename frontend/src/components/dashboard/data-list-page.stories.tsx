import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { DataListPage } from "./data-list-page";

interface DemoEntity {
  id: string;
  name: string;
}

const demoData: DemoEntity[] = [
  { id: "1", name: "Gruppe A" },
  { id: "2", name: "Gruppe B" },
];

const meta: Meta<typeof DataListPage<DemoEntity>> = {
  title: "components/dashboard/DataListPage",
  component: DataListPage<DemoEntity>,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;

type Story = StoryObj<typeof DataListPage<DemoEntity>>;

export const Empty: Story = {
  args: {
    title: "Kinderauswahl",
    sectionTitle: "Kinder auswählen",
    backUrl: "/dashboard",
    newEntityLabel: "Neues Kind erstellen",
    newEntityUrl: "/dashboard/new",
    data: [],
    onSelectEntityAction: fn(),
  },
};

export const WithData: Story = {
  args: {
    title: "Gruppenauswahl",
    sectionTitle: "Gruppen auswählen",
    backUrl: "/dashboard",
    newEntityLabel: "Neue Gruppe erstellen",
    newEntityUrl: "/dashboard/new",
    data: demoData,
    onSelectEntityAction: fn(),
  },
};
