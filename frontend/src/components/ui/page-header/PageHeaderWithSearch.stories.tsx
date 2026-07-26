import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { fn } from "storybook/test";
import { PageHeaderWithSearch } from "./PageHeaderWithSearch";
import type { ActiveFilter, FilterConfig } from "./types";

const meta: Meta<typeof PageHeaderWithSearch> = {
  title: "ui/page-header/PageHeaderWithSearch",
  component: PageHeaderWithSearch,
};

export default meta;
type Story = StoryObj<typeof PageHeaderWithSearch>;

const statusFilter: FilterConfig = {
  id: "status",
  label: "Status",
  type: "buttons",
  value: "active",
  onChange: fn(),
  options: [
    { value: "active", label: "Aktiv" },
    { value: "inactive", label: "Inaktiv" },
  ],
};

const activeFilters: ActiveFilter[] = [
  { id: "status", label: "Status: Aktiv", onRemove: fn() },
];

export const TitleOnly: Story = {
  args: {
    title: "Mitarbeitende",
  },
};

export const WithSearch: Story = {
  args: {
    title: "Kinder",
    search: {
      value: "",
      onChange: fn(),
      placeholder: "Suchen...",
    },
  },
};

export const WithFiltersAndChips: Story = {
  args: {
    title: "Kinder",
    search: {
      value: "",
      onChange: fn(),
      placeholder: "Suchen...",
    },
    filters: [statusFilter],
    activeFilters,
    onClearAllFilters: fn(),
  },
};

export const WithTabs: Story = {
  args: {
    tabs: {
      items: [
        { id: "overview", label: "Übersicht" },
        { id: "details", label: "Details", count: 3 },
      ],
      activeTab: "overview",
      onTabChange: fn(),
    },
  },
};

export const WithBadgeAndActionButton: Story = {
  args: {
    title: "Gruppen",
    badge: { count: 5, label: "Aktive Gruppen" },
    actionButton: (
      <button
        type="button"
        className="rounded-md bg-gray-900 px-3 py-1.5 text-sm text-white"
      >
        Neu
      </button>
    ),
  },
};

export const QuietFilterVariant: Story = {
  args: {
    title: "Kinder",
    search: {
      value: "",
      onChange: fn(),
      placeholder: "Suchen...",
    },
    filters: [statusFilter],
    filterVariant: "quiet",
  },
};

export const CompactOnScroll: Story = {
  args: {
    title: "Kinder",
    search: {
      value: "",
      onChange: fn(),
      placeholder: "Suchen...",
    },
    compactOnScroll: true,
  },
};

export const Interactive: Story = {
  render: () => {
    function InteractivePageHeader() {
      const [search, setSearch] = useState("");
      const [status, setStatus] = useState<string>("active");
      const [activeTab, setActiveTab] = useState("overview");

      const filters: FilterConfig[] = [
        {
          ...statusFilter,
          value: status,
          onChange: (value) => setStatus(value as string),
        },
      ];

      const currentActiveFilters: ActiveFilter[] =
        status === "active"
          ? []
          : [
              {
                id: "status",
                label: "Status: Inaktiv",
                onRemove: () => setStatus("active"),
              },
            ];

      return (
        <PageHeaderWithSearch
          title="Kinder"
          tabs={{
            items: [
              { id: "overview", label: "Übersicht" },
              { id: "details", label: "Details" },
            ],
            activeTab,
            onTabChange: setActiveTab,
          }}
          search={{
            value: search,
            onChange: setSearch,
            placeholder: "Suchen...",
          }}
          filters={filters}
          activeFilters={currentActiveFilters}
          onClearAllFilters={() => setStatus("active")}
        />
      );
    }

    return <InteractivePageHeader />;
  },
};
