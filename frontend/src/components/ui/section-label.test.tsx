import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { SectionLabel } from "./section-label";

describe("SectionLabel", () => {
  it("renders a level-3 heading by default", () => {
    render(<SectionLabel>Antworten</SectionLabel>);
    expect(
      screen.getByRole("heading", { level: 3, name: "Antworten" }),
    ).toBeInTheDocument();
  });

  it("names a fieldset as its legend", () => {
    render(
      <fieldset>
        <SectionLabel as="legend">Betreuungszeitraum</SectionLabel>
        <input aria-label="Beginn" />
      </fieldset>,
    );
    expect(
      screen.getByRole("group", { name: "Betreuungszeitraum" }),
    ).toContainElement(screen.getByLabelText("Beginn"));
  });

  it("marks a required group without changing its name", () => {
    render(
      <fieldset>
        <SectionLabel as="legend" required>
          Wochentage
        </SectionLabel>
      </fieldset>,
    );
    expect(screen.getByRole("group", { name: "Wochentage" })).toBeVisible();
    expect(screen.getByText("*", { exact: false })).toHaveAttribute(
      "aria-hidden",
      "true",
    );
  });

  it("renders a paragraph that is no heading", () => {
    render(<SectionLabel as="p">Betroffene Termine</SectionLabel>);
    expect(screen.queryByRole("heading")).not.toBeInTheDocument();
    expect(screen.getByText("Betroffene Termine").tagName).toBe("P");
  });

  it("passes id and aria attributes through", () => {
    render(
      <>
        <SectionLabel as="h2" id="tage">
          Tage
        </SectionLabel>
        <ul aria-labelledby="tage" />
      </>,
    );
    expect(screen.getByRole("list", { name: "Tage" })).toBeInTheDocument();
  });
});
