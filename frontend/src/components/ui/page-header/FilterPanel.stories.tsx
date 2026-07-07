import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { FilterPanel } from "./FilterPanel";
import type { FilterConfig, FilterSection } from "./types";

const meta: Meta<typeof FilterPanel> = {
  title: "ui/page-header/FilterPanel",
  component: FilterPanel,
};

export default meta;
type Story = StoryObj<typeof FilterPanel>;

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

const sections: FilterSection[] = [
  { title: "Allgemein", filterIds: ["status"] },
  { title: "Zuordnung", filterIds: ["group", "location"] },
];

export const Default: Story = {
  args: {
    isOpen: true,
    onClose: fn(),
    filters: [buttonsFilter, dropdownFilter, gridFilter],
    onApply: fn(),
    onReset: fn(),
  },
};

export const Quiet: Story = {
  args: {
    isOpen: true,
    onClose: fn(),
    filters: [buttonsFilter, dropdownFilter, gridFilter],
    variant: "quiet",
    onApply: fn(),
    onReset: fn(),
  },
};

export const Sectioned: Story = {
  args: {
    isOpen: true,
    onClose: fn(),
    filters: [buttonsFilter, dropdownFilter, gridFilter],
    variant: "quiet",
    sections,
    onApply: fn(),
    onReset: fn(),
  },
};

export const DesktopPlacement: Story = {
  args: {
    isOpen: true,
    onClose: fn(),
    filters: [buttonsFilter, dropdownFilter],
    placement: "desktop",
    anchorRect: { left: 100, bottom: 80, right: 300 },
    onApply: fn(),
    onReset: fn(),
  },
};

export const Closed: Story = {
  args: {
    isOpen: false,
    onClose: fn(),
    filters: [buttonsFilter],
  },
};
