import { describe, expect, it } from "vitest";
import type {
  HolidayKnownChild,
  HolidayRegistrationOffer,
} from "~/lib/holiday-registration-api";
import {
  type ChildDraft,
  emptyChild,
  emptyContact,
  emptyRegistration,
  hasRoomForKnownChild,
  knownChildRefused,
  OTHER_SCHOOL,
  payloadFromDraft,
  quoteAgeRanks,
  type RegistrationDraft,
  registrationFromFamily,
  toggleKnownChild,
  validateRegistration,
} from "./holiday-registration-model";
import { HOLIDAY_CONSENT_TEXTS } from "~/test/fixtures/holiday-consent-texts";

const offer: HolidayRegistrationOffer = {
  traegerName: "Träger Bissendorf",
  name: "Winterferien 2027",
  registrationOpensOn: "2026-11-02",
  registrationClosesOn: "2026-12-04",
  binding: true,
  holidayWishMode: "off",
  notes: "",
  open: true,
  weeks: [
    {
      id: "11",
      startsOn: "2027-01-04",
      endsOn: "2027-01-08",
      hostSchoolName: "OGS Nord",
      timeModels: [
        {
          id: "21",
          name: "bis 13 Uhr",
          startsAt: "08:00",
          endsAt: "13:00",
          priceCents: 4500,
        },
      ],
    },
    {
      id: "12",
      startsOn: "2027-01-11",
      endsOn: "2027-01-15",
      hostSchoolName: "OGS Süd",
      timeModels: [
        {
          id: "22",
          name: "bis 16 Uhr",
          startsAt: "08:00",
          endsAt: "16:00",
          priceCents: 6500,
        },
      ],
    },
  ],
  schools: [{ id: "31", name: "OGS Nord" }],
  captchaSiteKey: "site",
  consentTexts: HOLIDAY_CONSENT_TEXTS,
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
    contacts: [
      { ...emptyContact(), name: "Oma", phone: "0123", relationship: "" },
    ],
    privacyConsent: true,
    bindingConfirmed: true,
    custodyConfirmed: true,
    declarationName: " Eva Muster ",
  };
}

describe("validateRegistration", () => {
  it("refuses weeks that share only some days for one child (#5689)", () => {
    const first = offer.weeks[0]!;
    const partly = {
      ...first,
      id: "partly",
      startsOn: first.endsOn,
      endsOn: "2099-01-01",
    };
    const draft = emptyRegistration();
    const child = {
      ...draft.children[0]!,
      bookings: { [first.id]: "21", partly: "21" },
    };
    expect(
      validateRegistration(
        { ...draft, children: [child] },
        { ...offer, weeks: [first, partly] },
      )["children.0.bookings"],
    ).toBe("weeksOverlap");
  });

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
      "contacts.0.name": "required",
      "contacts.0.phone": "required",
      privacy_consent: "privacy",
      binding_confirmed: "binding",
      custody_confirmed: "custody",
      declaration_name: "declarationName",
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
      "contacts.0.name",
      "contacts.0.phone",
      "privacy_consent",
      "binding_confirmed",
      "custody_confirmed",
      "declaration_name",
    ]);
  });

  it("requires the custody and the name as the declaration (#5626)", () => {
    const draft = complete();
    expect(
      validateRegistration(
        { ...draft, custodyConfirmed: false, declarationName: "   " },
        offer,
      ),
    ).toEqual({
      custody_confirmed: "custody",
      declaration_name: "declarationName",
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

  it("sends the custody, the trimmed name and the version of the texts shown (#5626)", () => {
    const payload = payloadFromDraft(complete(), offer, "captcha");
    expect(payload.custody_confirmed).toBe(true);
    expect(payload.declaration_name).toBe("Eva Muster");
    expect(payload.consent_version).toBe(3);
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
    holidayWish: "",
    holidayWishProof: "",
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

describe("unified contacts (#5469)", () => {
  const person = (name: string, emergency: boolean, pickup: boolean) => ({
    ...emptyContact(),
    name,
    phone: " 0123 ",
    relationship: " Oma ",
    emergency,
    pickup,
  });

  it("sends both roles with identical details and preserves each list's order", () => {
    const draft = {
      ...complete(),
      contacts: [
        person("Pickup", false, true),
        person("Both", true, true),
        person("Emergency", true, false),
      ],
    };
    expect(validateRegistration(draft, offer)).toEqual({});
    const payload = payloadFromDraft(draft, offer, "captcha");
    expect(payload.emergency_contacts).toEqual([
      { name: "Both", phone: "0123", relationship: "Oma" },
      { name: "Emergency", phone: "0123", relationship: "Oma" },
    ]);
    expect(payload.pickup_persons).toEqual([
      { name: "Pickup", phone: "0123", relationship: "Oma" },
      { name: "Both", phone: "0123", relationship: "Oma" },
    ]);
  });

  it.each([
    [
      [person("No role", false, false), person("Emergency", true, false)],
      { "contacts.0.emergency": "contactRole" },
    ],
    [
      [person("Pickup", false, true)],
      { "contacts.0.emergency": "emergencyRequired" },
    ],
    [
      [{ ...person("Emergency", true, false), phone: " " }],
      { "contacts.0.phone": "required" },
    ],
    [
      Array.from({ length: 4 }, () => person("Emergency", true, false)),
      { "contacts.3.emergency": "emergencyLimit" },
    ],
    [
      [
        person("Emergency", true, false),
        ...Array.from({ length: 6 }, () => person("Pickup", false, true)),
      ],
      { "contacts.6.pickup": "pickupLimit" },
    ],
  ])("refuses invalid contact roles at their fields", (contacts, errors) => {
    expect(validateRegistration({ ...complete(), contacts }, offer)).toEqual(
      errors,
    );
  });

  it("allows three emergency contacts and five other pickup people", () => {
    expect(
      validateRegistration(
        {
          ...complete(),
          contacts: [
            ...Array.from({ length: 3 }, () =>
              person("Emergency", true, false),
            ),
            ...Array.from({ length: 5 }, () => ({
              ...person("Pickup", false, true),
              phone: "",
            })),
          ],
        },
        offer,
      ),
    ).toEqual({});
  });
});

describe("postal address of the parent (#5625)", () => {
  const asking = (addressRequirement: "off" | "optional" | "required") => ({
    ...offer,
    form: { ...offer.form, addressRequirement },
  });
  const withAddress = (
    street: string,
    postalCode: string,
    city: string,
  ): RegistrationDraft => ({
    ...complete(),
    guardianAddress: { street, postalCode, city },
  });

  it("asks nothing and sends nothing when the form does not ask", () => {
    const draft = withAddress("Musterweg 12", "49143", "Bissendorf");
    expect(validateRegistration(draft, asking("off"))).toEqual({});
    expect(payloadFromDraft(draft, asking("off"), "t")).not.toHaveProperty(
      "guardian_address",
    );
    expect(
      payloadFromDraft(draft, asking("off"), "t").guardian.first_name,
    ).toBe("Eva");
  });

  it("takes no address or all three parts when it is optional", () => {
    expect(validateRegistration(complete(), asking("optional"))).toEqual({});
    expect(
      payloadFromDraft(complete(), asking("optional"), "t"),
    ).not.toHaveProperty("guardian_address");
    expect(
      payloadFromDraft(complete(), asking("optional"), "t").form_version,
    ).toBe(1);
    expect(
      validateRegistration(
        withAddress("Musterweg 12", "", ""),
        asking("optional"),
      ),
    ).toEqual({
      "guardian_address.postal_code": "required",
      "guardian_address.city": "required",
    });
  });

  it("requires all three parts when it is required", () => {
    expect(validateRegistration(complete(), asking("required"))).toEqual({
      "guardian_address.street": "required",
      "guardian_address.postal_code": "required",
      "guardian_address.city": "required",
    });
  });

  it("refuses a postal code without five digits in both settings", () => {
    for (const requirement of ["optional", "required"] as const) {
      expect(
        validateRegistration(
          withAddress("Musterweg 12", "4914", "Bissendorf"),
          asking(requirement),
        ),
      ).toEqual({ "guardian_address.postal_code": "postalCode" });
    }
  });

  it("sends the trimmed address", () => {
    const draft = withAddress(" Musterweg 12 ", " 01234 ", " Bissendorf ");
    expect(validateRegistration(draft, asking("required"))).toEqual({});
    expect(
      payloadFromDraft(draft, asking("required"), "t").guardian_address,
    ).toEqual({
      street: "Musterweg 12",
      postal_code: "01234",
      city: "Bissendorf",
    });
  });
});

describe("order of the children by age for the quote (#5664)", () => {
  const born = (birthDate: string) => ({ birthDate }) as ChildDraft;

  it("names the oldest child first and keeps the place in the form", () => {
    expect(
      quoteAgeRanks([
        born("2018-03-01"),
        born("2016-09-12"),
        born("2017-01-20"),
      ]),
    ).toEqual([3, 1, 2]);
  });

  it("gives twins one place and a child without a date none", () => {
    expect(
      quoteAgeRanks([born("2017-05-05"), born("2017-05-05"), born("")]),
    ).toEqual([1, 1, 0]);
  });
});

describe("holiday care wish of a known child (#4703)", () => {
  const lia: HolidayKnownChild = {
    studentId: "41",
    schoolId: "31",
    firstName: "Lia",
    lastName: "Muster",
    birthDate: "2018-03-01",
    holidayWish: "yes",
    holidayWishProof: "sealed",
  };
  const withMode = (
    holidayWishMode: HolidayRegistrationOffer["holidayWishMode"],
  ): HolidayRegistrationOffer => ({ ...offer, holidayWishMode });

  it("sends the statement of a picked child of the Träger's school", () => {
    const draft = toggleKnownChild(complete(), lia, withMode("priority"));
    const payload = payloadFromDraft(draft, withMode("priority"), "t");
    const sent = payload.children.find((child) => child.first_name === "Lia");
    expect(sent?.home_school_id).toBe(31);
    expect(sent?.wish_proof).toBe("sealed");
  });

  it("sends no statement once the child is a holiday guest", () => {
    const picked = toggleKnownChild(complete(), lia, withMode("priority"));
    const guest = {
      ...picked,
      children: picked.children.map((child) =>
        child.firstName === "Lia" ? { ...child, school: OTHER_SCHOOL } : child,
      ),
    };
    const sent = payloadFromDraft(guest, withMode("priority"), "t").children;
    expect(sent.map((child) => child.first_name)).toContain("Lia");
    expect(sent.every((child) => child.wish_proof === undefined)).toBe(true);
  });

  it("refuses to pick a child without a wish only when the period admits only wishes", () => {
    const declined = { ...lia, holidayWish: "no" as const };
    expect(knownChildRefused(declined, withMode("wish_only"))).toBe(true);
    expect(knownChildRefused(declined, withMode("priority"))).toBe(false);
    expect(knownChildRefused(lia, withMode("wish_only"))).toBe(false);
    const draft = complete();
    expect(toggleKnownChild(draft, declined, withMode("wish_only"))).toBe(
      draft,
    );
  });
});

describe("booking of days (#4701)", () => {
  const byDays: HolidayRegistrationOffer = {
    ...offer,
    maxDaysPerChild: 3,
    weeks: [
      {
        ...offer.weeks[0]!,
        bookingUnit: "day",
        careDays: [
          "2027-01-04",
          "2027-01-05",
          "2027-01-06",
          "2027-01-07",
          "2027-01-08",
        ],
        fullDays: [],
      },
      offer.weeks[1]!,
    ],
  };
  const child = (patch: Partial<ChildDraft>): ChildDraft => ({
    ...emptyChild(),
    firstName: "Tom",
    lastName: "Tag",
    birthDate: "2017-05-04",
    school: "1",
    ...patch,
  });
  const errorsOf = (draftChild: ChildDraft) =>
    validateRegistration(
      { ...emptyRegistration(), children: [draftChild] },
      byDays,
    )["children.0.bookings"];

  it("needs a day in a week by days and sends the chosen days", () => {
    expect(errorsOf(child({ bookings: { "11": "21" } }))).toBe("days");
    const chosen = child({
      bookings: { "11": "21" },
      days: { "11": ["2027-01-05", "2027-01-04", "2027-01-10"] },
    });
    expect(errorsOf(chosen)).toBeUndefined();
    const payload = payloadFromDraft(
      { ...emptyRegistration(), children: [chosen] },
      byDays,
      "token",
    );
    expect(payload.children[0]?.bookings).toEqual([
      { week_id: 11, time_model_id: 21, days: ["2027-01-04", "2027-01-05"] },
    ]);
  });

  it("counts a whole week by its five care days against the bound", () => {
    expect(
      errorsOf(
        child({
          bookings: { "11": "21", "12": "22" },
          days: { "11": ["2027-01-04"] },
        }),
      ),
    ).toBe("dayLimit");
  });
});

describe("criteria of the allocation (#4700)", () => {
  const byCriteria: HolidayRegistrationOffer = {
    ...offer,
    allocationMode: "criteria",
    criteria: [
      { id: "1", question: "Sind beide Eltern berufstätig?", scope: "family" },
      {
        id: "2",
        question: "Ist ein Geschwisterkind angemeldet?",
        scope: "child",
      },
    ],
  };

  it("asks every family and every child criterion as a yes/no question", () => {
    expect(validateRegistration(complete(), byCriteria)).toEqual({
      "criteria.1": "yesNo",
      "children.0.criteria.2": "yesNo",
    });
    const answered: RegistrationDraft = {
      ...complete(),
      criteria: { "1": false },
      children: complete().children.map((child) => ({
        ...child,
        criteria: { "2": true },
      })),
    };
    expect(validateRegistration(answered, byCriteria)).toEqual({});
  });

  it("sends the answers per family and per child, a stale criterion left out", () => {
    const draft: RegistrationDraft = {
      ...complete(),
      criteria: { "1": true, "9": true },
      children: complete().children.map((child) => ({
        ...child,
        criteria: { "2": false },
      })),
    };
    const payload = payloadFromDraft(draft, byCriteria, "captcha");
    expect(payload.criteria).toEqual({ "1": true });
    expect(payload.children[0]?.criteria).toEqual({ "2": false });
  });

  it("asks and sends nothing for a period by arrival", () => {
    expect(validateRegistration(complete(), offer)).toEqual({});
    const payload = payloadFromDraft(complete(), offer, "captcha");
    expect(payload).not.toHaveProperty("criteria");
    expect(payload.children[0]).not.toHaveProperty("criteria");
  });
});
