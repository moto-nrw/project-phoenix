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

describe("optional second guardian", () => {
  it("omits a blank card and does not prefill it", () => {
    const draft = complete();
    expect(validateRegistration(draft, offer)).toEqual({});
    expect(
      payloadFromDraft(draft, offer, "captcha").second_guardian,
    ).toBeUndefined();
    expect(
      registrationFromFamily({ guardian: draft.guardian, children: [] })
        .secondGuardian,
    ).toEqual(emptyRegistration().secondGuardian);
  });

  it("trims the card and keeps contacts optional", () => {
    const draft = {
      ...complete(),
      secondGuardian: {
        firstName: " Tom ",
        lastName: " Anders ",
        email: " ",
        phone: " ",
      },
    };
    expect(validateRegistration(draft, offer)).toEqual({});
    expect(payloadFromDraft(draft, offer, "captcha").second_guardian).toEqual({
      first_name: "Tom",
      last_name: "Anders",
      email: "",
      phone: "",
    });
  });

  it.each([
    [
      { firstName: "Tom", lastName: "", email: "", phone: "" },
      { "second_guardian.last_name": "required" },
    ],
    [
      { firstName: "", lastName: "Anders", email: "", phone: "" },
      { "second_guardian.first_name": "required" },
    ],
    [
      { firstName: "Tom", lastName: "Anders", email: "bad", phone: "" },
      { "second_guardian.email": "email" },
    ],
    [
      { firstName: "Tom", lastName: "Anders", email: "", phone: "abc" },
      { "second_guardian.phone": "phone" },
    ],
  ])(
    "validates the second guardian with field paths",
    (secondGuardian, errors) => {
      expect(
        validateRegistration({ ...complete(), secondGuardian }, offer),
      ).toEqual(errors);
    },
  );
});
