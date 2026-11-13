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
  type ContactDraft,
  emptyContact,
  looksLikePhone,
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
const NAMED_RELATIONSHIP_TYPES = ["parent", "guardian", "relative"] as const;
export type NamedRelationshipType = (typeof NAMED_RELATIONSHIP_TYPES)[number];

export function isNamedRelationshipType(
  type: string,
): type is NamedRelationshipType {
  return (NAMED_RELATIONSHIP_TYPES as readonly string[]).includes(type);
}

/** Whether two sets of person fields are the same. */
function sameGuardian(
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

/**
 * Whether a field still holds the value the account brought. Each field
 * counts alone: a changed phone keeps the name marked as from the account.
 */
export function fromAccount(value: string, known: string | undefined): boolean {
  return known !== undefined && value.trim() !== "" && value === known;
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
  const empty = emptyContact();
  return (
    contact.prefilled === undefined &&
    contact.name.trim() === "" &&
    contact.phone.trim() === "" &&
    contact.relationship.trim() === "" &&
    contact.emergency === empty.emergency &&
    contact.pickup === empty.pickup
  );
}

/** The name a row was typed with, for a row the prefill did not write. */
function typedName(contact: ContactDraft): string {
  return contact.name.trim().toLowerCase();
}

/** The school records a row of the list stands for. */
function rowIds(contact: ContactDraft): readonly string[] {
  return (
    contact.knownPersonIds ??
    (contact.knownGuardianId ? [contact.knownGuardianId] : [])
  );
}

/**
 * A school number in the form the second custodian's check takes. The
 * school also stores brackets ("(05402) 1234", "+49 (0)5402 1234"); a number
 * the form would still refuse stays out, the parent types it.
 */
function formPhone(phone: string): string {
  const plain = phone
    .replace(/\(0\)/g, "")
    .replace(/[()]/g, "")
    .replace(/\s+/g, " ")
    .trim();
  return looksLikePhone(plain) ? plain : "";
}

/** A person of the picked children with all school records it has. */
interface KnownPerson extends HolidayKnownContact {
  /** One record per school; the first one names the person. */
  readonly ids: readonly string[];
  readonly schoolIds: readonly string[];
}

function personName(contact: HolidayKnownContact): string {
  return `${contact.firstName} ${contact.lastName}`.trim().toLowerCase();
}

/** A number in one spelling: digits only, "+49" and "0049" as "0". */
function phoneDigits(phone: string): string {
  const digits = phone.replace(/\(0\)/g, "").replace(/[^\d+]/g, "");
  return digits.replace(/^(\+|00)49/, "0").replace(/\+/g, "");
}

/** Whether both hold a value and the values differ. */
function contradicts(a: string, b: string): boolean {
  return a !== "" && b !== "" && a !== b;
}

/**
 * Whether a record of another school is the same person (#6682): siblings
 * at two schools of the Träger have one person under two records. Only the
 * name joins them there, so it joins only across schools and only while the
 * number and the email do not contradict. Two records of one school are
 * two persons, such as father and grandfather of the same name; a name a
 * school has twice joins nothing, since it cannot say which one is meant.
 */
function samePerson(
  person: KnownPerson,
  contact: HolidayKnownContact,
  schoolId: string,
  ambiguous: ReadonlySet<string>,
): boolean {
  return (
    personName(person) === personName(contact) &&
    !ambiguous.has(personName(contact)) &&
    !person.schoolIds.includes(schoolId) &&
    !contradicts(phoneDigits(person.phone), phoneDigits(contact.phone)) &&
    !contradicts(
      person.email.trim().toLowerCase(),
      contact.email.trim().toLowerCase(),
    )
  );
}

/** Names one school has under two or more records, over all children. */
function ambiguousNames(family: HolidayKnownFamily): Set<string> {
  const records = new Map<string, Set<string>>();
  for (const child of family.children) {
    for (const contact of child.contacts) {
      const key = `${child.schoolId}\u0000${personName(contact)}`;
      const ids = records.get(key) ?? new Set<string>();
      ids.add(contact.guardianProfileId);
      records.set(key, ids);
    }
  }
  const names = new Set<string>();
  for (const [key, ids] of records) {
    if (ids.size > 1) names.add(key.slice(key.indexOf("\u0000") + 1));
  }
  return names;
}

/**
 * The other guardians of the picked children, one entry per person: the
 * first child names the order, the roles of all picked children add up.
 */
function pickedContacts(
  draft: RegistrationDraft,
  family: HolidayKnownFamily,
): KnownPerson[] {
  const persons: KnownPerson[] = [];
  const ambiguous = ambiguousNames(family);
  for (const child of draft.children) {
    const known = family.children.find(
      (candidate) => candidate.studentId === child.knownStudentId,
    );
    if (!known) continue;
    for (const contact of known.contacts) {
      const index = persons.findIndex(
        (person) =>
          person.ids.includes(contact.guardianProfileId) ||
          samePerson(person, contact, known.schoolId, ambiguous),
      );
      const seen = persons[index];
      if (!seen) {
        persons.push({
          ...contact,
          ids: [contact.guardianProfileId],
          schoolIds: [known.schoolId],
        });
        continue;
      }
      persons[index] = {
        ...seen,
        ids: seen.ids.includes(contact.guardianProfileId)
          ? seen.ids
          : [...seen.ids, contact.guardianProfileId],
        schoolIds: seen.schoolIds.includes(known.schoolId)
          ? seen.schoolIds
          : [...seen.schoolIds, known.schoolId],
        email: seen.email || contact.email,
        phone: seen.phone || contact.phone,
        fullGuardian: seen.fullGuardian || contact.fullGuardian,
        emergency: seen.emergency || contact.emergency,
        pickup: seen.pickup || contact.pickup,
      };
    }
  }
  return persons;
}

/** The person of the picked children a row stands for, if any. */
function personOf(
  persons: readonly KnownPerson[],
  ids: readonly string[],
): KnownPerson | undefined {
  return persons.find((person) => person.ids.some((id) => ids.includes(id)));
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

/** The records of a row grow with the person, so a row keeps it. */
function withPersonIds(
  contact: ContactDraft,
  person: KnownPerson | undefined,
): ContactDraft {
  if (!person) return contact;
  const ids = [...new Set([...rowIds(contact), ...person.ids])];
  return ids.length === rowIds(contact).length
    ? contact
    : { ...contact, knownPersonIds: ids };
}

function syncContacts(
  current: readonly ContactDraft[],
  wanted: readonly KnownPerson[],
  dismissed: ReadonlySet<string>,
  label: RelationshipLabel,
): readonly ContactDraft[] {
  // By the person's records, not by name: two persons may share a name, and
  // either record of a person at two schools may be the one still picked.
  // A contact the parent typed or changed stays as it is. An untouched one
  // from the prefill follows the picked children: it takes their current
  // roles while they fit the limits, or goes when no picked child has it.
  const kept: ContactDraft[] = [];
  for (const [index, contact] of current.entries()) {
    const known = personOf(wanted, rowIds(contact));
    if (!isPrefilledContact(contact)) {
      kept.push(withPersonIds(contact, known));
      continue;
    }
    if (!known || !(known.emergency || known.pickup)) continue;
    const values = contactValues(known, label);
    kept.push(
      withPersonIds(
        fits(
          [...kept, ...current.slice(index + 1)].filter(
            (other) => !isBlankContact(other),
          ),
          values,
        )
          ? {
              ...contact,
              ...values,
              knownGuardianId: known.guardianProfileId,
              prefilled: values,
            }
          : contact,
        known,
      ),
    );
  }
  // A person already in the list gets no second row: a row from the
  // prefill by its records, a row the parent typed by its name.
  const presentIds = new Set(kept.flatMap(rowIds));
  const typedNames = new Set(
    kept
      .filter((contact) => rowIds(contact).length === 0)
      .map(typedName)
      .filter((name) => name),
  );
  let contacts = kept.filter((contact) => !isBlankContact(contact));
  for (const known of wanted) {
    if (!(known.emergency || known.pickup)) continue;
    if (known.ids.some((id) => presentIds.has(id))) continue;
    if (typedNames.has(personName(known))) continue;
    // The parent removed this person; another child does not bring it back.
    if (known.ids.some((id) => dismissed.has(id))) continue;
    const values = contactValues(known, label);
    if (!fits(contacts, values)) continue;
    contacts = [
      ...contacts,
      {
        ...emptyContact(),
        ...values,
        knownGuardianId: known.guardianProfileId,
        knownPersonIds: known.ids,
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
  wanted: readonly KnownPerson[],
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
    phone: formPhone(custodian.phone),
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
  // A removed person keeps out also under its record at another school.
  const dismissed = new Set(draft.dismissedContacts);
  for (const person of wanted) {
    if (person.ids.some((id) => dismissed.has(id))) {
      for (const id of person.ids) dismissed.add(id);
    }
  }
  return {
    ...draft,
    ...syncSecondGuardian(draft, wanted),
    contacts: syncContacts(draft.contacts, wanted, dismissed, label),
    ...(draft.dismissedContacts ? { dismissedContacts: [...dismissed] } : {}),
  };
}

/**
 * Takes the contacts the parent edited. A person from the account the parent
 * removed stays out when the choice of children changes later (#6682).
 */
export function withEditedContacts(
  draft: RegistrationDraft,
  contacts: readonly ContactDraft[],
): RegistrationDraft {
  const left = new Set(contacts.map((contact) => contact.knownGuardianId));
  // One person can come from siblings at two schools under two records, so
  // the removal keeps all records of the row.
  const removed = draft.contacts.flatMap((contact) =>
    contact.knownGuardianId &&
    contact.prefilled &&
    !left.has(contact.knownGuardianId)
      ? rowIds(contact)
      : [],
  );
  if (removed.length === 0) return { ...draft, contacts };
  return {
    ...draft,
    contacts,
    dismissedContacts: [...(draft.dismissedContacts ?? []), ...removed],
  };
}
