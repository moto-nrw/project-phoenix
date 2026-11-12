// Vorbefüllung der Ferienanmeldung aus dem Elternkonto (#6682): Anschrift,
// zweite sorgeberechtigte Person sowie Notfall- und Abholkontakte der
// gewählten Kinder, so wie die Schule sie kennt. Gesundheitsangaben gehören
// nicht dazu (Art. 9 DSGVO, Option A des Tickets). Die Werte sind eine Kopie:
// Eltern ändern sie vor dem Absenden, die Schuldaten bleiben unberührt.

import type {
  HolidayKnownContact,
  HolidayKnownFamily,
} from "~/lib/holiday-registration-api";
import {
  type AddressDraft,
  type ContactDraft,
  emptyContact,
  MAX_CONTACTS,
  MAX_EMERGENCY_CONTACTS,
  MAX_PICKUP_PERSONS,
  type RegistrationDraft,
} from "./holiday-registration-model";

/** The values of a contact the prefill wrote, to tell an edit apart. */
type ContactValues = Pick<
  ContactDraft,
  "name" | "phone" | "relationship" | "emergency" | "pickup"
>;

/** Translates a stored relationship type for the contact's free text. */
export type RelationshipLabel = (relationshipType: string) => string;

/** The relationship types the form names; "other" stays without a label. */
export const NAMED_RELATIONSHIP_TYPES = [
  "parent",
  "guardian",
  "relative",
] as const;
export type NamedRelationshipType = (typeof NAMED_RELATIONSHIP_TYPES)[number];

export function isNamedRelationshipType(
  type: string,
): type is NamedRelationshipType {
  return (NAMED_RELATIONSHIP_TYPES as readonly string[]).includes(type);
}

/** Whether two sets of person fields are the same. */
export function sameGuardian(
  a: RegistrationDraft["guardian"],
  b: RegistrationDraft["guardian"],
): boolean {
  return (
    a.firstName === b.firstName &&
    a.lastName === b.lastName &&
    a.email === b.email &&
    a.phone === b.phone
  );
}

/** Whether the address is still the one the prefill wrote. */
export function addressFromAccount(
  address: AddressDraft,
  family: HolidayKnownFamily | null,
): boolean {
  const known = family?.guardian.address;
  return (
    !!known &&
    address.street === known.street &&
    address.postalCode === known.postalCode &&
    address.city === known.city
  );
}

/** Whether the contact still holds exactly what the prefill wrote. */
export function isPrefilledContact(contact: ContactDraft): boolean {
  const known = contact.prefilled;
  return (
    !!known &&
    contact.name === known.name &&
    contact.phone === known.phone &&
    contact.relationship === known.relationship &&
    contact.emergency === known.emergency &&
    contact.pickup === known.pickup
  );
}

/** Whether the second custodian is still the one the prefill wrote. */
export function isPrefilledSecondGuardian(draft: RegistrationDraft): boolean {
  const known = draft.secondGuardianPrefill;
  return !!known && sameGuardian(draft.secondGuardian, known.values);
}

function isBlankContact(contact: ContactDraft): boolean {
  return (
    contact.prefilled === undefined &&
    contact.name.trim() === "" &&
    contact.phone.trim() === "" &&
    contact.relationship.trim() === ""
  );
}

/**
 * One key per person. Siblings at two schools of the Träger have the same
 * person under two school records, so the name joins them as well.
 */
function personKey(contact: HolidayKnownContact): string {
  return `${contact.firstName} ${contact.lastName}`.trim().toLowerCase();
}

/**
 * The other guardians of the picked children, one entry per person: the
 * first child names the order, the roles of all picked children add up.
 */
function pickedContacts(
  draft: RegistrationDraft,
  family: HolidayKnownFamily,
): HolidayKnownContact[] {
  const byPerson = new Map<string, HolidayKnownContact>();
  for (const child of draft.children) {
    const known = family.children.find(
      (candidate) => candidate.studentId === child.knownStudentId,
    );
    for (const contact of known?.contacts ?? []) {
      const key = personKey(contact);
      const seen = byPerson.get(key);
      byPerson.set(
        key,
        seen
          ? {
              ...seen,
              email: seen.email || contact.email,
              phone: seen.phone || contact.phone,
              fullGuardian: seen.fullGuardian || contact.fullGuardian,
              emergency: seen.emergency || contact.emergency,
              pickup: seen.pickup || contact.pickup,
            }
          : contact,
      );
    }
  }
  return [...byPerson.values()];
}

function contactValues(
  known: HolidayKnownContact,
  label: RelationshipLabel,
): ContactValues {
  return {
    name: `${known.firstName} ${known.lastName}`.trim(),
    phone: known.phone,
    relationship: label(known.relationshipType),
    emergency: known.emergency,
    pickup: known.pickup,
  };
}

/** Whether one more contact with these roles stays within the limits. */
function fits(contacts: readonly ContactDraft[], values: ContactValues) {
  const count = (role: "emergency" | "pickup") =>
    contacts.filter((contact) => contact[role]).length;
  return (
    contacts.length < MAX_CONTACTS &&
    (!values.emergency || count("emergency") < MAX_EMERGENCY_CONTACTS) &&
    (!values.pickup || count("pickup") < MAX_PICKUP_PERSONS)
  );
}

function syncContacts(
  current: readonly ContactDraft[],
  wanted: readonly HolidayKnownContact[],
  label: RelationshipLabel,
): readonly ContactDraft[] {
  const wantedIds = new Set(wanted.map((known) => known.guardianProfileId));
  // A contact the parent typed or changed stays; an untouched one from the
  // prefill goes when no picked child has it any more.
  const kept = current.filter(
    (contact) =>
      !isPrefilledContact(contact) ||
      wantedIds.has(contact.knownGuardianId ?? ""),
  );
  const present = new Set(kept.map((contact) => contact.knownGuardianId));
  let contacts = kept.filter((contact) => !isBlankContact(contact));
  for (const known of wanted) {
    if (!(known.emergency || known.pickup)) continue;
    if (present.has(known.guardianProfileId)) continue;
    const values = contactValues(known, label);
    if (!fits(contacts, values)) continue;
    contacts = [
      ...contacts,
      {
        ...emptyContact(),
        ...values,
        knownGuardianId: known.guardianProfileId,
        prefilled: values,
      },
    ];
  }
  return contacts.length > 0 ? contacts : [emptyContact()];
}

const blankGuardian: RegistrationDraft["guardian"] = {
  firstName: "",
  lastName: "",
  email: "",
  phone: "",
};

function syncSecondGuardian(
  draft: RegistrationDraft,
  wanted: readonly HolidayKnownContact[],
): Pick<RegistrationDraft, "secondGuardian" | "secondGuardianPrefill"> {
  const typed = Object.values(draft.secondGuardian).some(
    (value) => value.trim() !== "",
  );
  // The parent's own entry wins over the prefill, also an emptied one.
  const ownEntry = draft.secondGuardianPrefill
    ? !isPrefilledSecondGuardian(draft)
    : typed;
  if (ownEntry) {
    return {
      secondGuardian: draft.secondGuardian,
      secondGuardianPrefill: draft.secondGuardianPrefill,
    };
  }
  const custodian = wanted.find((known) => known.fullGuardian);
  if (!custodian) {
    return { secondGuardian: blankGuardian, secondGuardianPrefill: undefined };
  }
  const values = {
    firstName: custodian.firstName,
    lastName: custodian.lastName,
    email: custodian.email,
    phone: custodian.phone,
  };
  return {
    secondGuardian: values,
    secondGuardianPrefill: { guardianId: custodian.guardianProfileId, values },
  };
}

/**
 * Brings the second custodian and the contacts in line with the picked
 * children (#6682). Values from the prefill follow the choice of children;
 * what the parent typed or changed stays as it is.
 */
export function withKnownContacts(
  draft: RegistrationDraft,
  family: HolidayKnownFamily | null,
  label: RelationshipLabel,
): RegistrationDraft {
  if (!family) return draft;
  const wanted = pickedContacts(draft, family);
  return {
    ...draft,
    ...syncSecondGuardian(draft, wanted),
    contacts: syncContacts(draft.contacts, wanted, label),
  };
}
