import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { DemoShell } from "./demo-shell";

describe("DemoShell", () => {
  // #3603: every demo session is recorded; the visitor reads it on every
  // screen of the way in.
  it("tells the visitor that the demo is recorded", () => {
    render(
      <DemoShell>
        <p>Inhalt</p>
      </DemoShell>,
    );

    expect(
      screen.getByText(/werten wir aus, wie die Demo genutzt wird/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Mehr zum Datenschutz" }),
    ).toHaveAttribute("href", "https://moto-ogs.de/datenschutz");
  });
});
