import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import type { DataTableColumn } from "./data-table";
import { DataTable, DataTableStatusBadge } from "./data-table";

interface DemoRow {
  id: string;
  name: string;
  role: string;
  active: boolean;
}

const demoRows: DemoRow[] = [
  { id: "1", name: "Anna Schmidt", role: "Erzieherin", active: true },
  { id: "2", name: "Ben Fischer", role: "Praktikant", active: false },
  { id: "3", name: "Clara Weber", role: "Leitung", active: true },
];

const demoColumns: DataTableColumn<DemoRow>[] = [
  {
    key: "name",
    header: "Name",
    render: (row) => row.name,
    sortValue: (row) => row.name,
  },
  {
    key: "role",
    header: "Rolle",
    render: (row) => row.role,
    sortValue: (row) => row.role,
  },
  {
    key: "active",
    header: "Status",
    align: "right",
    render: (row) => <DataTableStatusBadge active={row.active} />,
  },
];

const meta = {
  title: "components/ui/DataTable",
  component: DataTable<DemoRow>,
} satisfies Meta<typeof DataTable<DemoRow>>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    columns: demoColumns,
    rows: demoRows,
    getRowKey: (row) => row.id,
  },
};

export const Loading: Story = {
  args: {
    columns: demoColumns,
    rows: [],
    getRowKey: (row) => row.id,
    isLoading: true,
  },
};

export const Empty: Story = {
  args: {
    columns: [],
    rows: [],
    getRowKey: (row: DemoRow) => row.id,
  },
};

export const WithHeaderActionsAndCaption: Story = {
  args: {
    columns: demoColumns,
    rows: demoRows,
    getRowKey: (row) => row.id,
    caption: "3 Mitarbeitende",
    headerActions: (
      <button type="button" className="text-sm text-gray-600">
        Filtern
      </button>
    ),
    defaultSortKey: "name",
  },
};

export const Paginated: Story = {
  args: {
    columns: demoColumns,
    rows: demoRows,
    getRowKey: (row) => row.id,
    pageSize: 2,
  },
};
