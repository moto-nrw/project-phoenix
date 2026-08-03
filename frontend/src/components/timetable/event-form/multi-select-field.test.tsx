import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { MultiSelectField } from "./multi-select-field";

describe("MultiSelectField protected values", () => {
  it("marks protected selections and keeps them when clearing the list", () => {
    const onChange = vi.fn();
    render(
      <MultiSelectField
        label="Kinder am Montag"
        options={[
          { id: "1", name: "Ada Anmeldung" },
          { id: "2", name: "Max Manuell" },
        ]}
        value={["1", "2"]}
        onChange={onChange}
        metadata="student"
        protectedValues={["1"]}
      />,
    );

    expect(
      screen.getByRole("checkbox", { name: /Ada Anmeldung.*aus Anmeldung/ }),
    ).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Auswahl leeren" }));

    expect(onChange).toHaveBeenCalledWith(["1"]);
  });
});
