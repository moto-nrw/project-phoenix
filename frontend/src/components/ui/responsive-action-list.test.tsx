import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import {
  ResponsiveActionList,
  ResponsiveActionRow,
} from "./responsive-action-list";

describe("ResponsiveActionRow", () => {
  it("renders each row as an item of the list", () => {
    render(
      <ResponsiveActionList id="hinweise">
        <ResponsiveActionRow>
          <span>„Fußball-AG“</span>
          <button type="button">Anlegen</button>
        </ResponsiveActionRow>
        <ResponsiveActionRow>
          <span>„Kunst“</span>
        </ResponsiveActionRow>
      </ResponsiveActionList>,
    );

    const list = screen.getByRole("list");
    expect(list).toHaveAttribute("id", "hinweise");
    const rows = within(list).getAllByRole("listitem");
    expect(rows).toHaveLength(2);
    expect(
      within(rows[0]!).getByRole("button", { name: "Anlegen" }),
    ).toBeInTheDocument();
  });

  it("joins the shared action column of the list from sm on", () => {
    render(
      <ResponsiveActionList>
        <ResponsiveActionRow>Zeile</ResponsiveActionRow>
      </ResponsiveActionList>,
    );

    expect(screen.getByRole("listitem")).toHaveClass(
      "sm:col-span-2",
      "sm:grid",
      "sm:grid-cols-subgrid",
    );
  });

  it("aligns the row to the top when asked", () => {
    render(
      <ResponsiveActionList>
        <ResponsiveActionRow align="start">Zeile</ResponsiveActionRow>
      </ResponsiveActionList>,
    );

    const row = screen.getByRole("listitem");
    expect(row).toHaveClass("sm:items-start");
    expect(row).not.toHaveClass("sm:items-center");
  });
});
