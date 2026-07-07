import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import React from "react";

import { SSEErrorBoundary } from "./SSEErrorBoundary";

const meta = {
  title: "components/sse/SSEErrorBoundary",
  component: SSEErrorBoundary,
} satisfies Meta<typeof SSEErrorBoundary>;

export default meta;
type Story = StoryObj<typeof meta>;

export const NoError: Story = {
  args: {
    children: (
      <div className="rounded-lg border border-gray-200 bg-white p-3 text-sm text-gray-700">
        Live-Updates aktiv.
      </div>
    ),
  },
};

function Thrower(): React.ReactElement {
  throw new Error("Simulated SSE failure");
}

export const CaughtErrorDefaultFallback: Story = {
  args: {
    children: <Thrower />,
  },
};

export const CaughtErrorCustomFallback: Story = {
  args: {
    children: <Thrower />,
    fallback: (
      <div className="rounded-lg border border-orange-200 bg-orange-50 p-3 text-sm text-orange-700">
        Benutzerdefinierter Fallback-Inhalt.
      </div>
    ),
  },
};
