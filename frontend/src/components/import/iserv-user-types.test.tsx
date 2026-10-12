import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import {
  IServUserTypes,
  selectedIServUserTypes,
  type IServSummary,
} from "./iserv-user-types";

const summary: IServSummary = {
  user_types: [
    { value: "Schüler(innen)", count: 2, selected: true },
    { value: "Lehrer(innen)", count: 1, selected: false },
    { value: "", count: 1, selected: false },
  ],
  deactivated_count: 1,
};

describe("IServUserTypes", () => {
  it("lists every user type with its count", () => {
    render(<IServUserTypes summary={summary} onChange={vi.fn()} />);

    expect(
      screen.getByRole("checkbox", { name: /Schüler\(innen\)/ }),
    ).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: /Lehrer\(innen\)/ }),
    ).not.toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: /Ohne Benutzertyp/ }),
    ).not.toBeChecked();
    expect(screen.getByText("2 Personen")).toBeInTheDocument();
    expect(
      screen.getByText("1 gesperrtes iServ-Konto wird nicht übernommen."),
    ).toBeInTheDocument();
  });

  it("adds and removes a user type", () => {
    const onChange = vi.fn();
    render(<IServUserTypes summary={summary} onChange={onChange} />);

    fireEvent.click(screen.getByRole("checkbox", { name: /Ohne Benutzertyp/ }));
    expect(onChange).toHaveBeenLastCalledWith(["Schüler(innen)", ""]);

    fireEvent.click(screen.getByRole("checkbox", { name: /Schüler\(innen\)/ }));
    expect(onChange).toHaveBeenLastCalledWith([]);
  });

  it("returns the chosen values", () => {
    expect(selectedIServUserTypes(summary)).toEqual(["Schüler(innen)"]);
  });
});
