import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { OfferingPreview } from "~/lib/roster-deviation";

import { OfferingPreviewList } from "./offering-preview-list";

// Schule am Berg chose only „Randstunde“ for class 1a. Children booked in a
// Ganztag offering dropped from the preview without a reason.
const preview: OfferingPreview = {
  from: "2026-11-04",
  weekdays: [
    {
      weekday: 1,
      added: [],
      removed: [
        { studentId: "1", name: "Benno Schmidt" },
        { studentId: "2", name: "Emil Thompson" },
      ],
      keptCount: 11,
    },
  ],
  segments: [],
  otherOfferings: [
    { offeringId: "6", name: "Ganztagsbetreuung bis 14.30 Uhr", childCount: 5 },
    { offeringId: "7", name: "Ganztagsbetreuung bis 16 Uhr", childCount: 3 },
  ],
};

describe("OfferingPreviewList", () => {
  it("names the offerings that keep dropped children and adds them", () => {
    const onAddOfferings = vi.fn();
    render(
      <OfferingPreviewList preview={preview} onAddOfferings={onAddOfferings} />,
    );

    expect(
      screen.getByText(
        "Kinder, die wegfallen, sind in anderen Angeboten angemeldet: „Ganztagsbetreuung bis 14.30 Uhr“ (5 Kinder) und „Ganztagsbetreuung bis 16 Uhr“ (3 Kinder).",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Kinder fallen weg, wenn sie an diesem Tag in keinem gewählten Angebot angemeldet sind.",
      ),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Angebote dazunehmen" }),
    );
    expect(onAddOfferings).toHaveBeenCalledWith(["6", "7"]);
  });

  it("names one offering in the singular", () => {
    render(
      <OfferingPreviewList
        preview={{
          ...preview,
          otherOfferings: [{ offeringId: "6", name: "Ganztag", childCount: 1 }],
        }}
        onAddOfferings={vi.fn()}
      />,
    );

    expect(
      screen.getByText(
        "Kinder, die wegfallen, sind in einem anderen Angebot angemeldet: „Ganztag“ (1 Kind).",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Angebot dazunehmen" }),
    ).toBeInTheDocument();
  });

  it("explains dropped children without another offering and offers no button", () => {
    render(
      <OfferingPreviewList
        preview={{ ...preview, otherOfferings: undefined }}
      />,
    );

    expect(screen.getByText(/Fällt weg \(2\)/)).toBeInTheDocument();
    expect(
      screen.getByText(
        "Kinder fallen weg, wenn sie an diesem Tag in keinem gewählten Angebot angemeldet sind.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});
