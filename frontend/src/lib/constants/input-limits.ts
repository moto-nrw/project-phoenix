/**
 * Default maximum character lengths for text-based form inputs.
 * Used by Input (`text`, `email`) and by DatabaseForm, which also applies
 * `textarea` to its multiline fields, to prevent layout breaks from
 * excessively long user input.
 *
 * The Textarea component itself sets no default: several of its fields allow
 * more than 2000 characters, e.g. legal texts of the enrollment form (no
 * backend limit) and contract letter settings (4000). A default would cut
 * them off (#6568).
 *
 * Individual fields can override these defaults via explicit maxLength props.
 */
const INPUT_MAX_LENGTHS = {
  text: 255,
  email: 255,
  textarea: 2000,
} as const;

type InputMaxLengthType = keyof typeof INPUT_MAX_LENGTHS;

export function getDefaultMaxLength(type: string): number | undefined {
  return INPUT_MAX_LENGTHS[type as InputMaxLengthType];
}
