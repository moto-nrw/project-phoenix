import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("~/components/ui/date-picker", async () =>
  (await import("~/test/mocks/date-picker")).datePickerModuleMock(),
);

import {
  QualifikationenFields,
  type StammdatenDraft,
} from "./stammdaten-section-forms";

const draft: StammdatenDraft = {
  firstName: "",
  lastName: "",
  birthday: "",
  gender: "",
  street: "",
  postalCode: "",
  city: "",
  phone: "",
  email: "",
  emergencyName: "",
  emergencyPhone: "",
  entryDate: "",
  contractEnd: "",
  probationEnd: "",
  weeklyHours: "",
  employmentType: "",
  qualifikationen: [
    {
      id: "1",
      typeId: "type-1",
      name: "",
      acquiredOn: "2026-04-20",
      expiresOn: "2028-04-20",
    },
  ],
  personnelNumber: "",
  financial: null,
};

describe("QualifikationenFields", () => {
  const types = [
    {
      id: "type-1",
      standardKey: null,
      name: "Erste Hilfe",
      validityMonths: 24,
      reminderLeadWeeks: 8,
      mandatory: false,
      archived: false,
    },
  ];

  it("allows an expiry date to be cleared despite the type's validity", () => {
    const onChange = vi.fn();
    render(
      <QualifikationenFields draft={draft} onChange={onChange} types={types} />,
    );

    fireEvent.change(screen.getByLabelText("Gültig bis"), {
      target: { value: "" },
    });

    expect(onChange).toHaveBeenCalledWith({
      qualifikationen: [expect.objectContaining({ expiresOn: "" })],
    });
  });

  it("suggests an expiry date when the acquisition date changes", () => {
    const onChange = vi.fn();
    render(
      <QualifikationenFields
        draft={{
          ...draft,
          qualifikationen: [
            { ...draft.qualifikationen[0]!, acquiredOn: "", expiresOn: "" },
          ],
        }}
        onChange={onChange}
        types={types}
      />,
    );

    fireEvent.change(screen.getByLabelText("Erworben am"), {
      target: { value: "2026-04-20" },
    });

    expect(onChange).toHaveBeenCalledWith({
      qualifikationen: [expect.objectContaining({ expiresOn: "2028-04-20" })],
    });
  });
});
