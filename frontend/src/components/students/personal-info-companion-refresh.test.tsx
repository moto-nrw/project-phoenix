import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, cleanup, waitFor, act } from "@testing-library/react";

/**
 * The Laufgemeinschaft is fetched separately from the student, so refreshing
 * the student after a save does NOT bring the links along. Without an explicit
 * revalidation the read-only Stammdaten card keeps rendering the pre-save list
 * for as long as it stays mounted (#1694).
 */

const { fetchStudentCompanionsMock } = vi.hoisted(() => ({
  fetchStudentCompanionsMock: vi.fn(),
}));

vi.mock("~/lib/student-companion-api", async (importOriginal) => {
  // Only the fetch is faked — subscribe/notify are the mechanism under test.
  const actual =
    await importOriginal<typeof import("~/lib/student-companion-api")>();
  return { ...actual, fetchStudentCompanions: fetchStudentCompanionsMock };
});

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: vi.fn() }),
}));

import { PersonalInfoReadOnly } from "./student-detail-components";
import { notifyStudentCompanionsChanged } from "~/lib/student-companion-api";
import type { ExtendedStudent } from "~/lib/hooks/use-student-data";

const student: ExtendedStudent = {
  id: "1",
  first_name: "Max",
  second_name: "Mustermann",
  name: "Max Mustermann",
  birthday: "2015-05-15",
  school_class: "3a",
  group_id: "g1",
  group_name: "Gruppe 1",
  buskind: false,
  pickup_status: "selbst",
  health_info: undefined,
  address_street: undefined,
  address_city: undefined,
  address_postal_code: undefined,
  supervisor_notes: undefined,
  extra_info: undefined,
  sick: false,
  sick_since: undefined,
  current_location: "Nicht anwesend",
  location_since: undefined,
  bus: false,
  bus_days: {},
};

describe("PersonalInfoReadOnly companion revalidation", () => {
  beforeEach(() => {
    fetchStudentCompanionsMock.mockReset();
  });

  afterEach(() => {
    cleanup();
  });

  it("refetches the links when a companion write is announced", async () => {
    fetchStudentCompanionsMock.mockResolvedValueOnce([
      {
        companion_student_id: "2",
        first_name: "Lina",
        last_name: "Beispiel",
        weekdays: ["mon"],
      },
    ]);

    render(<PersonalInfoReadOnly student={student} />);
    expect(await screen.findByText(/Lina Beispiel/)).toBeInTheDocument();

    // A save on this child — or the symmetric one on Lina's card — replaced the
    // links. The card is still mounted with the same student id.
    fetchStudentCompanionsMock.mockResolvedValueOnce([
      {
        companion_student_id: "3",
        first_name: "Tom",
        last_name: "Beispiel",
        weekdays: ["mon"],
      },
    ]);
    act(() => {
      notifyStudentCompanionsChanged();
    });

    expect(await screen.findByText(/Tom Beispiel/)).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.queryByText(/Lina Beispiel/)).not.toBeInTheDocument(),
    );
  });

  it("stops listening once unmounted", async () => {
    fetchStudentCompanionsMock.mockResolvedValue([]);
    const { unmount } = render(<PersonalInfoReadOnly student={student} />);
    await waitFor(() =>
      expect(fetchStudentCompanionsMock).toHaveBeenCalledTimes(1),
    );

    unmount();
    act(() => {
      notifyStudentCompanionsChanged();
    });

    expect(fetchStudentCompanionsMock).toHaveBeenCalledTimes(1);
  });
});
