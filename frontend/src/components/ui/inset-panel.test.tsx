import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { InsetPanel } from "./inset-panel";

describe("InsetPanel", () => {
  it("renders its content in a div by default", () => {
    render(<InsetPanel data-testid="panel">Vorschau</InsetPanel>);

    const panel = screen.getByTestId("panel");
    expect(panel.tagName).toBe("DIV");
    expect(panel).toHaveTextContent("Vorschau");
  });

  it("renders the requested element so lists and landmarks stay intact", () => {
    render(
      <InsetPanel as="section" aria-label="Summen">
        <p>Inhalt</p>
      </InsetPanel>,
    );
    expect(screen.getByRole("region", { name: "Summen" })).toBeInTheDocument();
  });

  it("keeps a list as a list", () => {
    render(
      <InsetPanel as="ul">
        <li>Eintrag</li>
      </InsetPanel>,
    );
    expect(screen.getByRole("listitem")).toHaveTextContent("Eintrag");
  });

  it("passes aria and data attributes through", () => {
    render(
      <InsetPanel role="status" aria-live="polite">
        Gespeichert
      </InsetPanel>,
    );
    expect(screen.getByRole("status")).toHaveAttribute("aria-live", "polite");
  });

  it("shows a frame by default except on the muted tone", () => {
    render(
      <>
        <InsetPanel data-testid="muted">A</InsetPanel>
        <InsetPanel data-testid="framed" framed>
          B
        </InsetPanel>
        <InsetPanel data-testid="white" tone="white">
          C
        </InsetPanel>
        <InsetPanel data-testid="danger" tone="danger">
          D
        </InsetPanel>
        <InsetPanel data-testid="warning" tone="warning">
          E
        </InsetPanel>
      </>,
    );

    expect(screen.getByTestId("muted")).not.toHaveClass("border");
    for (const id of ["framed", "white", "danger", "warning"]) {
      expect(screen.getByTestId(id)).toHaveClass("border");
    }
  });

  it("draws a dashed frame on the muted tone without framed", () => {
    render(
      <InsetPanel data-testid="panel" dashed>
        Leer
      </InsetPanel>,
    );
    expect(screen.getByTestId("panel")).toHaveClass("border", "border-dashed");
  });

  it("keeps the radius fixed and takes the padding step", () => {
    render(
      <>
        <InsetPanel data-testid="tight" padding="tight">
          A
        </InsetPanel>
        <InsetPanel data-testid="compact" padding="compact">
          B
        </InsetPanel>
        <InsetPanel data-testid="sm">A</InsetPanel>
        <InsetPanel data-testid="md" padding="md">
          C
        </InsetPanel>
      </>,
    );
    expect(screen.getByTestId("tight")).toHaveClass("px-3", "py-1");
    expect(screen.getByTestId("compact")).toHaveClass("px-3", "py-2");
    expect(screen.getByTestId("sm")).toHaveClass("rounded-xl", "p-3");
    expect(screen.getByTestId("md")).toHaveClass("rounded-xl", "p-4");
  });
});
