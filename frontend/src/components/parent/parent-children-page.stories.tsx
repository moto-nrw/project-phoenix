import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ParentChildrenPage } from "./parent-children-page";

const meta = {
  title: "components/parent/ParentChildrenPage",
  component: ParentChildrenPage,
} satisfies Meta<typeof ParentChildrenPage>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};
