import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import RelatedAccountsPanel from "./related-accounts-panel";

// The component fetches related accounts via `listRelatedAccounts` on mount.
// No API mocking layer (MSW) is configured for Storybook in this project, so
// the fetch will fail in the Storybook environment and the component's own
// error handling will surface its "could not load" message — this is
// expected and exercises the component's real error-state UI rather than a
// fabricated fixture.

const meta = {
  title: "components/parent/RelatedAccountsPanel",
  component: RelatedAccountsPanel,
  args: {
    studentId: "1",
    canInvite: true,
    canRemove: true,
  },
} satisfies Meta<typeof RelatedAccountsPanel>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const ReadOnly: Story = {
  args: {
    canInvite: false,
    canRemove: false,
  },
};
