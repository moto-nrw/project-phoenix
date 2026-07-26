import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { BrandLink, BreadcrumbDivider } from "./brand-link";

const meta = {
  title: "components/dashboard/header/BrandLink",
  component: BrandLink,
} satisfies Meta<typeof BrandLink>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {},
};

export const Scrolled: Story = {
  args: {
    isScrolled: true,
  },
};

export const CustomHref: Story = {
  args: {
    href: "/",
  },
};

export const Divider: Story = {
  render: () => <BreadcrumbDivider />,
};

export const Gallery: Story = {
  render: () => (
    <div className="flex items-center gap-4 bg-white p-4">
      <BrandLink />
      <BreadcrumbDivider />
      <BrandLink isScrolled />
    </div>
  ),
};
