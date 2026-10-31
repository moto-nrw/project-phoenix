import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { RosterDeviation } from "~/lib/roster-deviation";
import type { TimetableTemplate } from "~/lib/timetable-types";

import { TemplateList } from "./template-list";

function template(id: string, name: string): TimetableTemplate {
  return {
    id,
    name,
    type: "care",
    categoryId: "10",
    categoryName: "Betreuung",
    roomId: "5",
    roomName: "Raum 101",
    isOpen: true,
    maxParticipants: null,
    targetGroupType: "klasse",
    enrollmentCount: 20,
    supervisorCount: 1,
    requiredStaffCount: 0,
    assignedStaffCount: 0,
    studentIds: [],
    staffIds: [],
    weekdayAssignments: [],
    schedules: [
      {
        id: `s${id}`,
        weekday: 1,
        startTime: "07:00",
        endTime: "08:00",
        weekPattern: 0,
      },
    ],
    rosterMaintenance: {
      mode: "manual",
      offeringNames: [],
      gradeLevels: [],
      schoolClasses: [],
      inactiveOfferingNames: [],
      invalidOfferingNames: [],
      dynamicTargets: true,
      careOfferingsDisabled: false,
    },
  };
}

const deviation: RosterDeviation = {
  templateId: "2",
  offerings: [{ id: "3", name: "Randstunde" }],
  gradeLevels: [],
  schoolClasses: ["1b"],
  unbookedCount: 8,
  missingCount: 0,
  candidates: [{ id: "3", name: "Randstunde" }],
  combinations: [],
  ambiguous: false,
};

const noop = () => undefined;

function renderList(props: Partial<Parameters<typeof TemplateList>[0]> = {}) {
  return render(
    <TemplateList
      templates={[template("1", "Mensa"), template("2", "RDST 1b")]}
      deviations={new Map([["2", deviation]])}
      onCreate={noop}
      onEdit={noop}
      onApply={noop}
      onArchive={noop}
      onConnect={noop}
      onKeepManual={noop}
      reviewOnly={false}
      onReviewOnlyChange={noop}
      {...props}
    />,
  );
}

describe("TemplateList with roster deviations (#5103)", () => {
  it("lists the series to check first and shows the hint instead of the grey badge", () => {
    renderList();

    const headings = screen.getAllByRole("heading", { level: 3 });
    expect(headings.map((heading) => heading.textContent)).toEqual([
      "RDST 1b",
      "Mensa",
    ]);
    const deviating = headings[0]!.closest("article")!;
    expect(
      within(deviating).getByText(/Weicht von Anmeldungen ab/),
    ).toBeInTheDocument();
    expect(within(deviating).queryByText("Manuell")).not.toBeInTheDocument();
    const other = headings[1]!.closest("article")!;
    expect(within(other).getByText("Manuell")).toBeInTheDocument();
  });

  it("filters to the series to check", () => {
    const onReviewOnlyChange = vi.fn();
    const { rerender } = renderList({ onReviewOnlyChange });

    fireEvent.click(screen.getByRole("button", { name: "Zu prüfen (1)" }));
    expect(onReviewOnlyChange).toHaveBeenCalledWith(true);

    rerender(
      <TemplateList
        templates={[template("1", "Mensa"), template("2", "RDST 1b")]}
        deviations={new Map([["2", deviation]])}
        onCreate={noop}
        onEdit={noop}
        onApply={noop}
        onArchive={noop}
        reviewOnly
        onReviewOnlyChange={onReviewOnlyChange}
      />,
    );
    expect(
      screen.getAllByRole("heading", { level: 3 }).map((h) => h.textContent),
    ).toEqual(["RDST 1b"]);
  });

  it("passes the deviation to the connect action", () => {
    const onConnect = vi.fn();
    renderList({ onConnect });

    fireEvent.click(
      screen.getByRole("button", { name: "Mit Angebot verbinden" }),
    );
    expect(onConnect).toHaveBeenCalledWith(
      expect.objectContaining({ id: "2" }),
      deviation,
    );
  });
});
