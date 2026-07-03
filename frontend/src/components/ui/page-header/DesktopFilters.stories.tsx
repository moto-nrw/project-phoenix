import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { fn } from "storybook/test";
import { DesktopFilters } from "./DesktopFilters";
import type { FilterConfig } from "./types";

const meta: Meta<typeof DesktopFilters> = {
  title: "ui/page-header/DesktopFilters",
  component: DesktopFilters,
};

export default meta;
type Story = StoryObj<typeof DesktopFilters>;

const buttonsFilter: FilterConfig = {
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

const dropdownFilter: FilterConfig = {
  id: "group",
  label: "Gruppe",
  type: "dropdown",
  value: "all",
  onChange: fn(),
  options: [
    { value: "all", label: "Alle" },
    { value: "group-a", label: "Gruppe A", count: 12 },
    { value: "group-b", label: "Gruppe B", count: 8 },
  ],
};

const gridFilter: FilterConfig = {
  id: "location",
  label: "Ort",
  type: "grid",
  value: "all",
  onChange: fn(),
  options: [
    { value: "all", label: "Alle" },
    {
      value: "room",
      label: "Raum",
      icon: "M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6",
    },
  ],
};

export const Buttons: Story = {
  args: {
    filters: [buttonsFilter],
  },
};

export const Dropdown: Story = {
  args: {
    filters: [dropdownFilter],
  },
};

export const Grid: Story = {
  args: {
    filters: [gridFilter],
  },
};

export const Combined: Story = {
  args: {
    filters: [buttonsFilter, dropdownFilter, gridFilter],
  },
};

export const Interactive: Story = {
  render: () => {
    function InteractiveFilters() {
      const [status, setStatus] = useState<string>("active");
      const [group, setGroup] = useState<string>("all");

      const filters: FilterConfig[] = [
        {
          ...buttonsFilter,
          value: status,
          onChange: (value) => setStatus(value as string),
        },
        {
          ...dropdownFilter,
          value: group,
          onChange: (value) => setGroup(value as string),
        },
      ];

      return <DesktopFilters filters={filters} />;
    }

    return <InteractiveFilters />;
  },
};
