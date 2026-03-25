/**
 * Default maximum character lengths for text-based form inputs.
 * Used by Input, Textarea, and DatabaseForm components to prevent
 * layout breaks from excessively long user input.
 *
 * Individual fields can override these defaults via explicit maxLength props.
 */
export const INPUT_MAX_LENGTHS = {
  text: 255,
  email: 255,
  password: 128,
  textarea: 2000,
} as const;

export type InputMaxLengthType = keyof typeof INPUT_MAX_LENGTHS;

export function getDefaultMaxLength(type: string): number | undefined {
  return INPUT_MAX_LENGTHS[type as InputMaxLengthType];
}
