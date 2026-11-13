import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { expect, screen, userEvent, within } from "storybook/test";

import { AnchoredPopover } from "./anchored-popover";
import { Button } from "./button";
import { CustomSelect } from "./custom-select";

function Demo() {
  const [open, setOpen] = useState(false);
  return (
    <div className="p-12">
      <AnchoredPopover
        open={open}
        onOpenChange={setOpen}
        ariaLabel="Zeitraum wählen"
        header={<p className="text-sm font-semibold text-gray-900">Zeitraum</p>}
        renderTrigger={({ ref, open: isOpen, panelId, toggle }) => (
          <Button
            ref={ref}
            type="button"
            variant="outline"
            size="md"
            aria-expanded={isOpen}
            aria-controls={panelId}
            onClick={toggle}
          >
            Öffnen
          </Button>
        )}
      >
        {() => (
          <p className="p-4 text-sm text-gray-700">
            Diese Woche, vom 07.09. bis 11.09.2026
          </p>
        )}
      </AnchoredPopover>
    </div>
  );
}

function SelectDemo() {
  const [open, setOpen] = useState(false);
  const [value, setValue] = useState("woche");
  return (
    <div className="p-12">
      <AnchoredPopover
        open={open}
        onOpenChange={setOpen}
        ariaLabel="Ansicht wählen"
        footer={
          <p className="px-4 py-3 text-sm text-gray-600">Gilt für alle</p>
        }
        renderTrigger={({ ref, open: isOpen, panelId, toggle }) => (
          <Button
            ref={ref}
            type="button"
            variant="outline"
            size="md"
            aria-expanded={isOpen}
            aria-controls={panelId}
            onClick={toggle}
          >
            Ansicht
          </Button>
        )}
      >
        {() => (
          <div className="p-4">
            <CustomSelect
              ariaLabel="Zeitraum"
              value={value}
              onChange={setValue}
              options={[
                { value: "tag", label: "Tag" },
                { value: "woche", label: "Woche" },
                { value: "monat", label: "Monat" },
              ]}
            />
          </div>
        )}
      </AnchoredPopover>
    </div>
  );
}

const meta = {
  title: "components/ui/AnchoredPopover",
  component: Demo,
  parameters: { layout: "fullscreen" },
} satisfies Meta<typeof Demo>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Closed: Story = {};

export const Opened: Story = {
  play: async ({ canvasElement }) => {
    await userEvent.click(
      within(canvasElement).getByRole("button", { name: "Öffnen" }),
    );
    await expect(
      await screen.findByRole("dialog", { name: "Zeitraum wählen" }),
    ).toBeVisible();
  },
};

/** Auswahlfeld im Popover, Liste offen über dem Popover (#6576, Baseline). */
export const SelectInsideOpened: Story = {
  render: () => <SelectDemo />,
  play: async ({ canvasElement }) => {
    await userEvent.click(
      within(canvasElement).getByRole("button", { name: "Ansicht" }),
    );
    await userEvent.click(
      await screen.findByRole("combobox", { name: "Zeitraum" }),
    );
    await expect(await screen.findByRole("listbox")).toBeVisible();
  },
};
