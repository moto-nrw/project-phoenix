import { describe, expect, it } from "vitest";
import type {
  HolidayKnownChild,
  HolidayRegistrationOffer,
} from "~/lib/holiday-registration-api";
import {
  emptyChild,
  emptyContact,
  emptyRegistration,
  hasRoomForKnownChild,
  OTHER_SCHOOL,
  payloadFromDraft,
  type RegistrationDraft,
  registrationFromFamily,
  toggleKnownChild,
  validateRegistration,
} from "./holiday-registration-model";

const offer: HolidayRegistrationOffer = {
  traegerName: "Träger Bissendorf",
  name: "Winterferien 2027",
  registrationOpensOn: "2026-11-02",
  registrationClosesOn: "2026-12-04",
  binding: true,
  notes: "",
  open: true,
  weeks: [
    {
      id: "11",
      startsOn: "2027-01-04",
      endsOn: "2027-01-08",
      hostSchoolName: "OGS Nord",
      timeModels: [
        { id: "21", name: "bis 13 Uhr", startsAt: "08:00", endsAt: "13:00" },
      ],
    },
    {
      id: "12",
      startsOn: "2027-01-11",
      endsOn: "2027-01-15",
      hostSchoolName: "OGS Süd",
      timeModels: [
        { id: "22", name: "bis 16 Uhr", startsAt: "08:00", endsAt: "16:00" },
      ],
    },
  ],
  schools: [{ id: "31", name: "OGS Nord" }],
  captchaSiteKey: "site",
  form: { version: 1, fields: [] },
};

function complete(): RegistrationDraft {
  const draft = emptyRegistration();
  const [child] = draft.children;
  return {
    ...draft,
    guardian: {
      firstName: " Eva ",
      lastName: "Muster",
      email: "eva@example.org",
      phone: "0151 1",
    },
    children: [
      {
        ...child!,
        firstName: "Ben",
        lastName: "Muster",
        birthDate: "2017-05-04",
        school: OTHER_SCHOOL,
        guestSchoolName: " GS Melle ",
        bookings: { "12": "22", "11": "21" },
      },
    ],
    emergencyContacts: [
      { ...emptyContact(), name: "Oma", phone: "0123", relationship: "" },
    ],
    privacyConsent: true,
    bindingConfirmed: true,
  };
}

describe("validateRegistration", () => {
  it("names every missing field of an empty registration", () => {
    expect(validateRegistration(emptyRegistration(), offer)).toEqual({
      "guardian.first_name": "required",
      "guardian.last_name": "required",
      "guardian.phone": "required",
      "guardian.email": "required",
      "children.0.first_name": "required",
      "children.0.last_name": "required",
      "children.0.birth_date": "required",
      "children.0.home_school_id": "school",
      "children.0.bookings": "weeks",
      "emergency_contacts.0.name": "required",
      "emergency_contacts.0.phone": "required",
      privacy_consent: "privacy",
      binding_confirmed: "binding",
    });
  });

  it("names the missing fields in the order of the form", () => {
    // The form focuses the first refused field; it must be the first one
    // on the screen, not the parent's own fields below the children.
    expect(
      Object.keys(validateRegistration(emptyRegistration(), offer)),
    ).toEqual([
      "children.0.first_name",
      "children.0.last_name",
      "children.0.birth_date",
      "children.0.home_school_id",
      "children.0.bookings",
      "guardian.first_name",
      "guardian.last_name",
      "guardian.email",
      "guardian.phone",
      "emergency_contacts.0.name",
      "emergency_contacts.0.phone",
      "privacy_consent",
      "binding_confirmed",
    ]);
  });

  it("asks for the health consent only when a child has health information", () => {
    const draft = complete();
    expect(validateRegistration(draft, offer)).toEqual({});
    const withAllergy: RegistrationDraft = {
      ...draft,
      children: [{ ...draft.children[0]!, allergies: "Nüsse" }],
    };
    expect(validateRegistration(withAllergy, offer)).toEqual({
      health_consent: "health",
    });
  });

  it("refuses an address that is no e-mail address", () => {
    const draft = complete();
    expect(
      validateRegistration(
        { ...draft, guardian: { ...draft.guardian, email: "eva@" } },
        offer,
      ),
    ).toEqual({ "guardian.email": "email" });
  });
});

describe("payloadFromDraft", () => {
  it("sends a holiday guest without a school and the weeks in their order", () => {
    const payload = payloadFromDraft(complete(), offer, "captcha");
    expect(payload.guardian.first_name).toBe("Eva");
    expect(payload.children[0]).toEqual(
      expect.objectContaining({
        home_school_id: 0,
        guest_school_name: "GS Melle",
        bookings: [
          { week_id: 11, time_model_id: 21 },
          { week_id: 12, time_model_id: 22 },
        ],
      }),
    );
    expect(payload.health_consent).toBe(false);
    expect(payload.binding_confirmed).toBe(true);
    expect(payload.captcha_token).toBe("captcha");
  });

  it("sends the school of a child of the Träger without a guest school name", () => {
    const draft = complete();
    const payload = payloadFromDraft(
      { ...draft, children: [{ ...draft.children[0]!, school: "31" }] },
      offer,
      "captcha",
    );
    expect(payload.children[0]?.home_school_id).toBe(31);
    expect(payload.children[0]?.guest_school_name).toBe("");
  });
});

describe("known children of a parent account (#4662)", () => {
  const felix: HolidayKnownChild = {
    studentId: "41",
    schoolId: "31",
    firstName: "Felix",
    lastName: "Schneider",
    birthDate: "2018-03-01",
  };
  const known = (studentId: string): HolidayKnownChild => ({
    ...felix,
    studentId,
  });

  it("prefills the parent fields and nothing else", () => {
    const draft = registrationFromFamily({
      guardian: {
        firstName: "Sabine",
        lastName: "Schneider",
        email: "s@example.org",
        phone: "0151",
      },
      children: [felix],
    });
    expect(draft.guardian.email).toBe("s@example.org");
    expect(draft.children).toHaveLength(1);
    expect(draft.children[0]!.firstName).toBe("");
    // The prefill answers none of the Träger's questions (#5198).
    expect(draft.answers).toEqual({});
    expect(draft.children[0]!.answers).toEqual({});
    expect(registrationFromFamily(null).guardian.email).toBe("");
  });

  it("puts a picked child in place of the empty child and drops it again", () => {
    const picked = toggleKnownChild(emptyRegistration(), felix, offer);
    expect(picked.children).toHaveLength(1);
    expect(picked.children[0]).toEqual(
      expect.objectContaining({
        firstName: "Felix",
        birthDate: "2018-03-01",
        school: "31",
        knownStudentId: "41",
      }),
    );
    const dropped = toggleKnownChild(picked, felix, offer);
    expect(dropped.children).toHaveLength(1);
    expect(dropped.children[0]!.knownStudentId).toBeUndefined();
  });

  it("keeps a typed child and leaves an unknown school for the parent", () => {
    const typed: RegistrationDraft = {
      ...emptyRegistration(),
      children: [{ ...emptyChild(), firstName: "Ben" }],
    };
    const picked = toggleKnownChild(typed, { ...felix, schoolId: "99" }, offer);
    expect(picked.children.map((child) => child.firstName)).toEqual([
      "Ben",
      "Felix",
    ]);
    expect(picked.children[1]!.school).toBe("");
  });

  it("adds no fourth child", () => {
    let draft = emptyRegistration();
    for (const id of ["1", "2", "3"]) {
      draft = toggleKnownChild(draft, known(id), offer);
    }
    expect(hasRoomForKnownChild(draft)).toBe(false);
    expect(toggleKnownChild(draft, known("4"), offer)).toBe(draft);
  });

  it("sends no reference to the school record", () => {
    const draft = toggleKnownChild(emptyRegistration(), felix, offer);
    const payload = payloadFromDraft(draft, offer, "token");
    expect(Object.keys(payload.children[0]!)).not.toContain("known_student_id");
    expect(JSON.stringify(payload)).not.toContain("41");
  });
});

describe("answers to the Träger's questions (#5198)", () => {
  const withQuestions: HolidayRegistrationOffer = {
    ...offer,
    form: {
      version: 2,
      fields: [
        {
          key: "allein",
          label: "Darf allein nach Hause gehen",
          type: "boolean",
          applies_to_child: true,
          sort_order: 0,
        },
        {
          key: "uhrzeit",
          label: "Ab wann?",
          type: "text",
          required: true,
          applies_to_child: true,
          sort_order: 1,
          visible_when: {
            source: "field",
            field: "allein",
            operator: "eq",
            value: true,
          },
        },
        {
          key: "hinweis",
          label: "Hinweis",
          type: "information",
          content: "Bitte lesen",
          sort_order: 2,
        },
      ],
    },
  };

  it("drops the stale answer of a hidden question and sends no information block", () => {
    const draft = complete();
    const child = {
      ...draft.children[0]!,
      answers: { allein: false, uhrzeit: "15 Uhr", hinweis: "x" },
    };
    const next = { ...draft, children: [child] };
    expect(validateRegistration(next, withQuestions)).toEqual({});
    const payload = payloadFromDraft(next, withQuestions, "captcha");
    expect(payload.children[0]?.custom_data).toEqual({ allein: false });
    expect(payload.form_version).toBe(2);
    expect(payload.custom_data).toEqual({});
  });

  it("requires the shown follow-up question with its request path", () => {
    const draft = complete();
    const child = { ...draft.children[0]!, answers: { allein: true } };
    expect(
      validateRegistration({ ...draft, children: [child] }, withQuestions),
    ).toEqual({ "children.0.custom_data.uhrzeit": "required" });
  });
});
