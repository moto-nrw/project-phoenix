import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ParentRealtimeBridge } from "~/components/parent/parent-realtime-bridge";

const meta: Meta<typeof ParentRealtimeBridge> = {
  title: "components/parent/ParentRealtimeBridge",
  component: ParentRealtimeBridge,
};

export default meta;
type Story = StoryObj<typeof ParentRealtimeBridge>;

// Renders nothing — this story only confirms the SSE bridge mounts and
// subscribes without throwing outside its normal app-shell context.
export const Default: Story = {};
