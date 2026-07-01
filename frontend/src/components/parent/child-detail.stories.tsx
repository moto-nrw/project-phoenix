import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { BreadcrumbProvider } from "~/lib/breadcrumb-context";
import { ChildDetail } from "./child-detail";

const meta = {
  title: "components/parent/ChildDetail",
  component: ChildDetail,
  decorators: [
    (Story) => (
      <BreadcrumbProvider>
        <Story />
      </BreadcrumbProvider>
    ),
  ],
} satisfies Meta<typeof ChildDetail>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    studentId: "1",
  },
};
