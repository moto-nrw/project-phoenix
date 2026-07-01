import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import type { DateRange } from "react-day-picker";
import {
  DateRangePicker,
  RangeCalendarInline,
  buildDefaultPresets,
} from "./date-range-picker";

const meta: Meta<typeof DateRangePicker> = {
  title: "components/ui/DateRangePicker",
  component: DateRangePicker,
};

export default meta;
type Story = StoryObj<typeof DateRangePicker>;

function DateRangePickerDemo() {
  const [value, setValue] = useState<DateRange | undefined>(undefined);
  const today = new Date();
  return (
    <div className="p-10">
      <DateRangePicker
        value={value}
        onChange={setValue}
        presets={buildDefaultPresets(
          new Date(today.getFullYear(), 0, 1),
          today,
        )}
      />
    </div>
  );
}

export const Default: Story = {
  render: () => <DateRangePickerDemo />,
};

export const WithSelectedRange: Story = {
  render: () => {
    const today = new Date();
    const [value, setValue] = useState<DateRange | undefined>({
      from: today,
      to: today,
    });
    return (
      <div className="p-10">
        <DateRangePicker value={value} onChange={setValue} />
      </div>
    );
  },
};

export const NoPresets: Story = {
  render: () => {
    const [value, setValue] = useState<DateRange | undefined>(undefined);
    return (
      <div className="p-10">
        <DateRangePicker value={value} onChange={setValue} />
      </div>
    );
  },
};

export const RangeCalendarInlineStandalone: StoryObj<
  typeof RangeCalendarInline
> = {
  render: () => {
    const [value, setValue] = useState<DateRange | undefined>(undefined);
    return (
      <div className="relative p-10">
        <RangeCalendarInline value={value} onChange={setValue} />
      </div>
    );
  },
};
