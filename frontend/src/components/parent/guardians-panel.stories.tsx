import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import GuardiansPanel from "~/components/parent/guardians-panel";

// GuardiansPanel fetches its data client-side via listChildGuardians() on
// mount. There is no MSW/mock-server wiring in this Storybook setup, so the
// fetch will fail against the storybook host — the component handles that
// gracefully by showing the shared load error with a retry in each section,
// which is still a faithful render of a real failure state.
const meta = {
  title: "components/parent/GuardiansPanel",
  component: GuardiansPanel,
  parameters: {
    layout: "padded",
  },
  args: {
    studentId: "1",
    canInvite: true,
    canRemove: true,
  },
} satisfies Meta<typeof GuardiansPanel>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
