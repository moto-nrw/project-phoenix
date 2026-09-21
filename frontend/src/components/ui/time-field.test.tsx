import { useState } from "react";
import { expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TimeField } from "./time-field";

function TimeFieldDemo({ initialValue }: Readonly<{ initialValue: string }>) {
  const [value, setValue] = useState(initialValue);

  return (
    <TimeField
      value={value}
      onChange={setValue}
      label="Uhrzeit"
      hint="Uhrzeit im Format 15:30"
      placeholder="HH:MM"
    />
  );
}

it("keeps a completed time when another digit is typed", async () => {
  const user = userEvent.setup();
  render(<TimeFieldDemo initialValue="12:34" />);

  const input = screen.getByPlaceholderText("HH:MM");
  await user.type(input, "5");

  expect(input).toHaveValue("12:34");
});

it("continues a sequential one-digit-hour entry", async () => {
  const user = userEvent.setup();
  render(<TimeFieldDemo initialValue="" />);

  const input = screen.getByPlaceholderText("HH:MM");
  await user.type(input, "1530");

  expect(input).toHaveValue("15:30");
});
