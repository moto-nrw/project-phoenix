import { InsetPanel } from "~/components/ui/inset-panel";

interface InvitationSummaryProps {
  readonly email: string;
  /** Name der eingeladenen Person. Die E-Mail-Adresse steht dann darunter. */
  readonly name?: string;
  /** Anzeigename der Rolle, in die eingeladen wird. */
  readonly role?: string;
  /** Übersetzter Einleitungstext; ohne Angabe „Einladung für“. */
  readonly label?: string;
}

/** Sagt auf einer Einladungsseite, für wen die Einladung gilt. */
export function InvitationSummary({
  email,
  name,
  role,
  label = "Einladung für",
}: InvitationSummaryProps) {
  return (
    <InsetPanel as="section" framed className="text-sm">
      <p className="font-medium text-gray-700">
        {label}{" "}
        <span className="font-semibold text-gray-900">{name || email}</span>
        {role ? (
          <>
            {" "}
            als <span className="font-semibold text-gray-900">{role}</span>
          </>
        ) : null}
      </p>
      {name ? <p className="mt-1 text-gray-500">{email}</p> : null}
    </InsetPanel>
  );
}
