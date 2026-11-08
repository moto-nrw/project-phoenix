// Contact formats the backend accepts for guardians
// (`backend/modules/peopledirectory/contact/validation.go`). The enrollment
// form checks them up front so a value it accepts is never rejected later
// when the child is taken over on approval.

const EMAIL_PATTERN = /^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+[.][A-Za-z]+$/;
const PHONE_PATTERN = /^(\+[0-9]{1,3}\s?)?[0-9\s-]{7,15}$/;

export function isValidContactEmail(value: string): boolean {
  return EMAIL_PATTERN.test(value);
}

export function isValidContactPhone(value: string): boolean {
  return PHONE_PATTERN.test(value);
}
