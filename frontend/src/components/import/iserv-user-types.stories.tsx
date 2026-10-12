import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { IServUserTypes } from "./iserv-user-types";

const meta = {
  title: "components/import/IServUserTypes",
  component: IServUserTypes,
  args: { onChange: () => undefined },
} satisfies Meta<typeof IServUserTypes>;

export default meta;

type Story = StoryObj<typeof meta>;

export const PupilsPreselected: Story = {
  args: {
    summary: {
      user_types: [
        { value: "Schüler(innen)", count: 212, selected: true },
        { value: "Lehrer(innen)", count: 18, selected: false },
        { value: "", count: 3, selected: false },
      ],
      deactivated_count: 7,
    },
  },
};

export const NothingChosen: Story = {
  args: {
    summary: {
      user_types: [
        { value: "Kinder", count: 180, selected: false },
        { value: "Personal", count: 25, selected: false },
      ],
      deactivated_count: 0,
    },
  },
};
