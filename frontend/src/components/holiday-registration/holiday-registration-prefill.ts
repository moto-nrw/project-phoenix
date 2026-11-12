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
  type RegistrationDraft,
} from "./holiday-registration-model";

// Mirrors MAX_EMERGENCY_CONTACTS / MAX_PICKUP_PERSONS of the model.
const MAX_EMERGENCY = 3;
const MAX_PICKUP = 5;

/** The values of a contact the prefill wrote, to tell an edit apart. */
type ContactValues = Pick<
  ContactDraft,
  "name" | "phone" | "relationship" | "emergency" | "pickup"
>;

/** Translates a stored relationship type for the contact's free text. */
export type RelationshipLabel = (relationshipType: string) => string;

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
  if (!known) return false;
  const current = draft.secondGuardian;
  return (
    current.firstName === known.values.firstName &&
    current.lastName === known.values.lastName &&
    current.email === known.values.email &&
    current.phone === known.values.phone
  );
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
 * The other guardians of the picked children, one entry per person: the
 * first child names the order, the roles of all picked children add up.
 */
function pickedContacts(
  draft: RegistrationDraft,
  family: HolidayKnownFamily,
): HolidayKnownContact[] {
  const byId = new Map<string, HolidayKnownContact>();
  for (const child of draft.children) {
    const known = family.children.find(
      (candidate) => candidate.studentId === child.knownStudentId,
    );
    for (const contact of known?.contacts ?? []) {
      const seen = byId.get(contact.guardianProfileId);
      byId.set(
        contact.guardianProfileId,
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
  return [...byId.values()];
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
    (!values.emergency || count("emergency") < MAX_EMERGENCY) &&
    (!values.pickup || count("pickup") < MAX_PICKUP)
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
