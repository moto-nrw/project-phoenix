import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { EmptyStudentResults } from "./empty-student-results";

const meta = {
  title: "components/ui/EmptyStudentResults",
  component: EmptyStudentResults,
} satisfies Meta<typeof EmptyStudentResults>;

export default meta;

type Story = StoryObj<typeof meta>;

export const NoStudentsAtAll: Story = {
  args: {
    totalCount: 0,
    filteredCount: 0,
  },
};

export const FilteredOutAllResults: Story = {
  args: {
    totalCount: 42,
    filteredCount: 0,
  },
};
