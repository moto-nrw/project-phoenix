import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { StudentConsentsReadOnly } from "./student-consents-read-only";

describe("StudentConsentsReadOnly", () => {
  it("zeigt hinterlegte und widerrufene Einwilligungen ohne Aktion", () => {
    render(
      <StudentConsentsReadOnly
        consents={[
          {
            key: "agb",
            state: "granted",
            changed_at: "2026-08-20T09:00:00Z",
          },
          { key: "data_processing", state: "not_recorded" },
          { key: "email_contact", state: "not_recorded" },
          {
            key: "photo",
            state: "withdrawn",
            changed_at: "2026-08-31T15:00:00Z",
          },
        ]}
      />,
    );

    expect(
      screen.getByRole("heading", {
        name: "Einwilligungen und Bestätigungen",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Allgemeine Geschäftsbedingungen (AGB)"),
    ).toBeInTheDocument();
    expect(screen.getByText("Foto-Einwilligung")).toBeInTheDocument();
    expect(screen.getByText("Hinterlegt am 20.08.2026")).toBeInTheDocument();
    expect(screen.getByText("Widerrufen am 31.08.2026")).toBeInTheDocument();
    expect(
      screen.queryByText("Datenschutz zur Kenntnis genommen"),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });

  it("blendet den Bereich ohne hinterlegte Einwilligungen aus", () => {
    const { container } = render(
      <StudentConsentsReadOnly
        consents={[
          { key: "agb", state: "not_recorded" },
          { key: "data_processing", state: "not_recorded" },
          { key: "email_contact", state: "not_recorded" },
          { key: "photo", state: "not_recorded" },
        ]}
      />,
    );

    expect(container).toBeEmptyDOMElement();
  });
});
