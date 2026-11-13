import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { FieldShell, useFieldIds } from "./field-shell";

function Field({
  id,
  label,
  hint,
  error,
  errorLive,
  describedBy,
  keepWrapper,
}: Readonly<{
  id?: string;
  label?: string;
  hint?: string;
  error?: string;
  errorLive?: boolean;
  describedBy?: string;
  keepWrapper?: boolean;
}>) {
  const ids = useFieldIds({ id, hint, error, describedBy });
  return (
    <FieldShell
      ids={ids}
      label={label}
      required
      hint={hint}
      error={error}
      errorLive={errorLive}
      keepWrapper={keepWrapper}
    >
      <input
        id={ids.id}
        aria-describedby={ids.describedBy}
        data-testid="control"
      />
    </FieldShell>
  );
}

describe("FieldShell", () => {
  it("names the control by its label, also without an id of the caller", () => {
    render(<Field label="Vorname" />);

    expect(screen.getByRole("textbox", { name: "Vorname" })).toBe(
      screen.getByTestId("control"),
    );
  });

  it("describes the control by the caller's text, the hint and the error, in that order", () => {
    render(
      <>
        <p id="outer">Außen</p>
        <Field
          id="first-name"
          label="Vorname"
          hint="Wie im Ausweis."
          error="Bitte ausfüllen."
          describedBy="outer"
        />
      </>,
    );

    expect(
      screen.getByRole("textbox", { name: "Vorname" }),
    ).toHaveAccessibleDescription("Außen Wie im Ausweis. Bitte ausfüllen.");
  });

  it("reads the error out at once only when asked to", () => {
    const { rerender } = render(
      <Field label="Vorname" error="Bitte ausfüllen." />,
    );
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.getByText("Bitte ausfüllen.")).toBeInTheDocument();

    rerender(<Field label="Vorname" error="Bitte ausfüllen." errorLive />);
    expect(screen.getByRole("alert")).toHaveTextContent("Bitte ausfüllen.");
  });

  it("marks a required field with the star next to the label, not in its name", () => {
    render(<Field label="Vorname" />);

    expect(screen.getByText("*")).toBeInTheDocument();
    expect(
      screen.getByRole("textbox", { name: "Vorname" }),
    ).toBeInTheDocument();
  });

  it("renders no wrapper for a bare control unless asked to", () => {
    const { container, rerender } = render(<Field />);
    expect(container.firstElementChild).toBe(screen.getByTestId("control"));

    rerender(<Field keepWrapper />);
    expect(container.firstElementChild?.tagName).toBe("DIV");
    expect(container.firstElementChild).toContainElement(
      screen.getByTestId("control"),
    );
  });
});
