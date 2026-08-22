import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { BreadcrumbProvider } from "~/lib/breadcrumb-context";
import { ChildMasterDataView } from "./child-master-data";

/**
 * ChildMasterDataView fetches its data from `/api/parent/*` on mount
 * (getChildMasterData / getChildFeatures). Storybook has no backend or
 * route handlers wired up, so the fetch rejects and the component settles
 * into its own error state — this is the component's real, intentional
 * error-handling path (see `if (error || !data || !features)` in the
 * source), not a broken story.
 */
const meta = {
  title: "parent/ChildMasterDataView",
  component: ChildMasterDataView,
  decorators: [
    (Story) => (
      <BreadcrumbProvider>
        <Story />
      </BreadcrumbProvider>
    ),
  ],
} satisfies Meta<typeof ChildMasterDataView>;

export default meta;

type Story = StoryObj<typeof meta>;

export const LoadErrorState: Story = {
  args: {
    studentId: "storybook-student-1",
    childName: "Lina Muster",
  },
};
