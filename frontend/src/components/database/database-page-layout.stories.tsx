import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { DatabasePageLayout } from "./database-page-layout";

const meta = {
  title: "components/database/DatabasePageLayout",
  component: DatabasePageLayout,
} satisfies Meta<typeof DatabasePageLayout>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    loading: false,
    sessionLoading: false,
    children: (
      <div className="p-8">
        <h1 className="text-xl font-semibold">Database page content</h1>
        <p className="text-gray-600">
          Rendered inside DatabasePageLayout when neither the session nor the
          page data is loading.
        </p>
      </div>
    ),
  },
};

export const Loading: Story = {
  args: {
    loading: true,
    sessionLoading: false,
    children: <div>Content</div>,
  },
};

export const SessionLoading: Story = {
  args: {
    loading: false,
    sessionLoading: true,
    children: <div>Content</div>,
  },
};
