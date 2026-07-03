import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Avatar } from "./avatar";

const meta: Meta<typeof Avatar> = {
  title: "components/ui/Avatar",
  component: Avatar,
};

export default meta;
type Story = StoryObj<typeof Avatar>;

export const Initials: Story = {
  args: {
    name: "Anna Musterfrau",
    size: "md",
  },
};

export const WithImage: Story = {
  args: {
    name: "Max Mustermann",
    imageUrl: "/help/screens/mitarbeiter.webp",
    size: "md",
  },
};

export const BrokenImageFallsBackToInitials: Story = {
  args: {
    name: "Erika Beispiel",
    imageUrl: "/does-not-exist.jpg",
    size: "md",
  },
};

export const Sizes: Story = {
  render: () => (
    <div className="flex items-end gap-4">
      <Avatar name="Anna Musterfrau" size="xs" />
      <Avatar name="Anna Musterfrau" size="sm" />
      <Avatar name="Anna Musterfrau" size="md" />
      <Avatar name="Anna Musterfrau" size="lg" />
      <Avatar name="Anna Musterfrau" size="xl" />
    </div>
  ),
};
