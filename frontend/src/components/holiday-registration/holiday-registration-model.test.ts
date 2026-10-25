import { describe, expect, it } from "vitest";
import type { HolidayRegistrationOffer } from "~/lib/holiday-registration-api";
import {
  emptyContact,
  emptyRegistration,
  OTHER_SCHOOL,
  payloadFromDraft,
  type RegistrationDraft,
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
