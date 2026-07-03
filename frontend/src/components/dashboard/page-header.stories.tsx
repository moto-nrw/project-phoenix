import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { PageHeader } from "./page-header";

const meta = {
  title: "components/dashboard/PageHeader",
  component: PageHeader,
} satisfies Meta<typeof PageHeader>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    title: "Dashboard",
  },
};

export const WithDescription: Story = {
  args: {
    title: "Schülerverwaltung",
    description: "Verwalte alle Schülerinnen und Schüler deiner Einrichtung",
  },
};

export const WithoutBackButton: Story = {
  args: {
    title: "Startseite",
    backUrl: "",
  },
};
