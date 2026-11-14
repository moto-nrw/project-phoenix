/**
 * An e-mail address that breaks only after its „@“: never inside a name or
 * at the hyphen of a domain, where a wrapped address reads as two.
 */
export function EmailText({ email }: Readonly<{ email: string }>) {
  const at = email.indexOf("@");
  if (at < 0) return <span className="whitespace-nowrap">{email}</span>;
  return (
    <>
      <span className="whitespace-nowrap">{email.slice(0, at + 1)}</span>
      <wbr />
      <span className="whitespace-nowrap">{email.slice(at + 1)}</span>
    </>
  );
}
