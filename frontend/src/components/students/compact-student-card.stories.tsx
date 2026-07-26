import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { CompactStudentCard } from "./compact-student-card";

const meta = {
  title: "students/CompactStudentCard",
  component: CompactStudentCard,
  parameters: {
    layout: "centered",
  },
  args: {
    studentId: 1,
    firstName: "Mia",
    lastName: "Schmidt",
    schoolClass: "3a",
    groupName: "Bärengruppe",
  },
  decorators: [
    (Story) => (
      <div className="w-80">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof CompactStudentCard>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Clickable: Story = {
  args: {
    onClick: () => {
      // no-op for story purposes
    },
  },
};

export const PlainChrome: Story = {
  args: {
    chrome: "plain",
  },
};

export const NoMeta: Story = {
  args: {
    schoolClass: undefined,
    groupName: undefined,
  },
};

export const WithPhoto: Story = {
  args: {
    photoUrl: "https://placekitten.com/200/200",
  },
};
