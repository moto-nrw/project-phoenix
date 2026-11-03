/**
 * The length limit of a text setting (`Validation.MaxLength`, #5622). The
 * backend counts characters (Unicode code points); the browser's
 * `maxLength` counts UTF-16 units and would refuse an emoji at the limit.
 * Returns the hint for a text over the limit, otherwise null.
 */
export function textTooLong(text: string, maxLength: number): string | null {
  return [...text].length > maxLength
    ? `Der Text ist zu lang. Erlaubt sind ${maxLength} Zeichen.`
    : null;
}
