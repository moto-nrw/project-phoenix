import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { DatePicker } from "./date-picker";

const meta: Meta<typeof DatePicker> = {
  title: "components/ui/DatePicker",
  component: DatePicker,
};

export default meta;
type Story = StoryObj<typeof DatePicker>;

function SingleDemo() {
  const [value, setValue] = useState<Date | null>(null);
  return (
    <div className="p-10">
      <DatePicker value={value} onChange={setValue} />
    </div>
  );
}

function SingleWithValueDemo() {
  const [value, setValue] = useState<Date | null>(new Date());
  return (
    <div className="p-10">
      <DatePicker value={value} onChange={setValue} />
    </div>
  );
}

function MultipleDemo() {
  const [values, setValues] = useState<Date[]>([]);
  return (
    <div className="p-10">
      <DatePicker mode="multiple" values={values} onChangeDates={setValues} />
    </div>
  );
}

function InlineDemo() {
  const [value, setValue] = useState<Date | null>(null);
  return (
    <div className="p-10">
      <DatePicker value={value} onChange={setValue} calendarLayout="inline" />
    </div>
  );
}

export const Default: Story = {
  render: () => <SingleDemo />,
};

export const WithSelectedValue: Story = {
  render: () => <SingleWithValueDemo />,
};

export const MultipleMode: Story = {
  render: () => <MultipleDemo />,
};

export const InlineLayout: Story = {
  render: () => <InlineDemo />,
};
