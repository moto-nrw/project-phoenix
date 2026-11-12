import { describe, expect, it } from "vitest";
import type {
  HolidayKnownChild,
  HolidayKnownContact,
  HolidayKnownFamily,
  HolidayRegistrationOffer,
} from "~/lib/holiday-registration-api";
import {
  emptyContact,
  registrationFromFamily,
  toggleKnownChild,
} from "./holiday-registration-model";
import {
  addressFromAccount,
  isPrefilledContact,
  isPrefilledSecondGuardian,
  withKnownContacts,
} from "./holiday-registration-prefill";

const label = (type: string) => (type === "relative" ? "Verwandte" : "");

const contact = (
  id: string,
  patch: Partial<HolidayKnownContact> = {},
): HolidayKnownContact => ({
  guardianProfileId: id,
  firstName: `Vorname${id}`,
  lastName: "Muster",
  email: "",
  phone: `0151 ${id}`,
  relationshipType: "relative",
  fullGuardian: false,
  emergency: true,
  pickup: false,
  ...patch,
});

const child = (
  id: string,
  contacts: HolidayKnownContact[],
): HolidayKnownChild => ({
  studentId: id,
  schoolId: "1",
  firstName: `Kind${id}`,
  lastName: "Muster",
  birthDate: "2018-03-01",
  holidayWish: "",
  holidayWishProof: "",
  contacts,
});

const coParent = contact("20", {
  firstName: "Jonas",
  email: "jonas@example.org",
  relationshipType: "parent",
  fullGuardian: true,
  emergency: true,
});
const grandma = contact("21", { firstName: "Erika", pickup: true });
const neighbour = contact("22", {
  firstName: "Nora",
  phone: "",
  emergency: false,
  pickup: true,
});

const family: HolidayKnownFamily = {
  guardian: {
    firstName: "Sabine",
    lastName: "Muster",
    email: "sabine@example.org",
    phone: "0151 1",
    address: { street: "Lindenweg 4", postalCode: "49143", city: "Bissendorf" },
  },
  children: [
    child("10", [coParent, grandma]),
    child("11", [{ ...grandma, emergency: false }, neighbour]),
  ],
  accountProof: "",
};

const offer = {
  schools: [{ id: "1", name: "Schule" }],
  holidayWishMode: "off",
} as unknown as HolidayRegistrationOffer;

const pick = (
  draft: ReturnType<typeof registrationFromFamily>,
  known: HolidayKnownChild,
) => withKnownContacts(toggleKnownChild(draft, known, offer), family, label);

describe("prefill from the parent account (#6682)", () => {
  it("takes the address the school knows", () => {
    const draft = registrationFromFamily(family);
    expect(draft.guardianAddress).toEqual(family.guardian.address);
    expect(addressFromAccount(draft.guardianAddress, family)).toBe(true);
    expect(
      addressFromAccount(
        { ...draft.guardianAddress, street: "Neuer Weg 1" },
        family,
      ),
    ).toBe(false);
  });

  it("fills the second custodian and the contacts of a picked child", () => {
    const draft = pick(registrationFromFamily(family), family.children[0]!);

    expect(draft.secondGuardian).toEqual({
      firstName: "Jonas",
      lastName: "Muster",
      email: "jonas@example.org",
      phone: "0151 20",
    });
    expect(isPrefilledSecondGuardian(draft)).toBe(true);
    // The blank first contact makes room for the known ones.
    expect(draft.contacts.map((c) => c.name)).toEqual([
      "Jonas Muster",
      "Erika Muster",
    ]);
    expect(draft.contacts[1]).toMatchObject({
      phone: "0151 21",
      relationship: "Verwandte",
      emergency: true,
      pickup: true,
      knownGuardianId: "21",
    });
    expect(draft.contacts.every(isPrefilledContact)).toBe(true);
    // Health never comes from the school.
    expect(draft.children[0]).toMatchObject({
      allergies: "",
      medication: "",
      healthNotes: "",
    });
  });

  it("lists a person of two siblings once, with the roles of both", () => {
    let draft = pick(registrationFromFamily(family), family.children[0]!);
    draft = pick(draft, family.children[1]!);
    expect(draft.contacts.map((c) => c.knownGuardianId)).toEqual([
      "20",
      "21",
      "22",
    ]);
    expect(draft.contacts[1]).toMatchObject({ emergency: true, pickup: true });
  });

  it("drops untouched values with the child and keeps the parent's changes", () => {
    let draft = pick(registrationFromFamily(family), family.children[0]!);
    draft = {
      ...draft,
      contacts: draft.contacts.map((c) =>
        c.knownGuardianId === "21" ? { ...c, phone: "0170 99" } : c,
      ),
    };
    draft = pick(draft, family.children[0]!);

    expect(draft.secondGuardian.firstName).toBe("");
    expect(draft.secondGuardianPrefill).toBeUndefined();
    expect(draft.contacts).toHaveLength(1);
    expect(draft.contacts[0]).toMatchObject({
      knownGuardianId: "21",
      phone: "0170 99",
    });
    expect(isPrefilledContact(draft.contacts[0]!)).toBe(false);
  });

  it("leaves a second custodian the parent typed or emptied", () => {
    const typed = {
      ...registrationFromFamily(family),
      secondGuardian: {
        firstName: "Tom",
        lastName: "Anders",
        email: "",
        phone: "",
      },
    };
    expect(pick(typed, family.children[0]!).secondGuardian.firstName).toBe(
      "Tom",
    );

    const filled = pick(registrationFromFamily(family), family.children[0]!);
    const emptied = {
      ...filled,
      secondGuardian: { firstName: "", lastName: "", email: "", phone: "" },
    };
    expect(pick(emptied, family.children[1]!).secondGuardian.firstName).toBe(
      "",
    );
  });

  it("joins one person of siblings at two schools", () => {
    const twoSchools: HolidayKnownFamily = {
      ...family,
      children: [
        child("10", [grandma]),
        {
          ...child("13", [{ ...grandma, guardianProfileId: "91" }]),
          schoolId: "2",
        },
      ],
    };
    let draft = withKnownContacts(
      toggleKnownChild(
        registrationFromFamily(twoSchools),
        twoSchools.children[0]!,
        offer,
      ),
      twoSchools,
      label,
    );
    draft = withKnownContacts(
      toggleKnownChild(draft, twoSchools.children[1]!, offer),
      twoSchools,
      label,
    );
    expect(draft.contacts.map((c) => c.name)).toEqual(["Erika Muster"]);
  });

  it("keeps the limits of the form", () => {
    const many = Array.from({ length: 5 }, (_, i) => contact(`3${i}`));
    const crowded = { ...family, children: [child("12", many)] };
    const draft = withKnownContacts(
      toggleKnownChild(
        registrationFromFamily(crowded),
        crowded.children[0]!,
        offer,
      ),
      crowded,
      label,
    );
    expect(draft.contacts.filter((c) => c.emergency)).toHaveLength(3);
  });

  it("starts with one empty contact when no child brings any", () => {
    const draft = withKnownContacts(
      { ...registrationFromFamily(family), contacts: [emptyContact()] },
      family,
      label,
    );
    expect(draft.contacts).toHaveLength(1);
    expect(draft.contacts[0]!.knownGuardianId).toBeUndefined();
    expect(withKnownContacts(draft, null, label)).toBe(draft);
  });
});
