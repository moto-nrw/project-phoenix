import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { RequestReviewCard, RequestRowHeader } from "./request-review-card";

describe("RequestReviewCard", () => {
  it("keeps an open enrollment request in the four-column open grid", () => {
    render(
      <RequestReviewCard
        childName="Lina Beispiel"
        type="enrollment"
        submittedAt="2026-08-20T09:00:00Z"
        history={{ kind: "readonly", label: "Offen", tone: "orange" }}
      />,
    );

    const row = screen.getByRole("button", {
      name: /Anfrage für Lina Beispiel.*Anmeldung.*Offen.*Details anzeigen/,
    });
    expect(row).toHaveClass(
      "sm:grid-cols-[minmax(0,11rem)_minmax(0,1fr)_auto_1rem]",
    );
    expect(row.children).toHaveLength(4);
  });

  it("exposes the column labels to assistive technology", () => {
    render(<RequestRowHeader view="open" />);

    const header = screen.getByText("Wartet seit").parentElement;
    expect(header).not.toHaveAttribute("aria-hidden");
  });

  it("shows the collapsed summary on mobile", () => {
    render(
      <RequestReviewCard
        childName="Lina Beispiel"
        type="care_schedule"
        typeLabel="Einzelner Tag"
        summary="26.08.2026 · Abholzeit"
      />,
    );

    expect(screen.getByText("26.08.2026 · Abholzeit")).not.toHaveClass(
      "hidden",
    );
  });

  it("does not repeat the child name inside a grouped case", () => {
    render(
      <RequestReviewCard
        childName="Lina Beispiel"
        type="excused"
        typeLabel="Krankmeldung"
        summary="29.08.2026"
        grouped
      />,
    );

    expect(screen.queryByText("Lina Beispiel")).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", {
        name: /Anfrage für Lina Beispiel.*Krankmeldung.*Details anzeigen/,
      }),
    ).toBeVisible();
  });
});
