// Mails of a holiday registration that did not reach the parents (#6737):
// the receipt and the acceptance mail. The office learns it here, so it can
// check the address and reach the family another way.
import { Alert } from "~/components/ui/alert";
import { StatusBadge } from "~/components/ui/status-badge";
import type {
  HolidayRegistrationMailKind,
  HolidayRegistrationMailProblem,
} from "~/lib/traeger-holiday-registrations-api";

// The receipt is no acceptance yet; its name keeps it apart from the
// office's „Bestätigen“.
const MAIL_NAMES: Record<HolidayRegistrationMailKind, string> = {
  holiday_registration_received: "die Eingangsbestätigung",
  holiday_registration_accepted: "die Annahme-Mail",
};

/** One sentence per mail: which mail and what became of it. */
export function holidayMailProblemText(
  problem: HolidayRegistrationMailProblem,
): string {
  const mail = MAIL_NAMES[problem.kind];
  const Mail = mail.charAt(0).toUpperCase() + mail.slice(1);
  switch (problem.state) {
    case "blocked":
      return `${Mail} wurde nicht gesendet. Die E-Mail-Adresse ist gesperrt.`;
    case "unknown":
      return `Ob ${mail} angekommen ist, ist unklar.`;
    case "failed":
      return `${Mail} hat die Eltern nicht erreicht.`;
  }
}

/** The notice in the registration: every mail with a problem and what to do. */
export function HolidayRegistrationMailProblemsAlert({
  problems,
}: Readonly<{ problems: readonly HolidayRegistrationMailProblem[] }>) {
  if (problems.length === 0) return null;
  return (
    <Alert
      type="warning"
      title="E-Mail an die Eltern prüfen"
      message={
        <>
          {problems.map((problem) => (
            <span key={problem.kind} className="block">
              {holidayMailProblemText(problem)}
            </span>
          ))}
          <span className="block">
            Prüfen Sie die E-Mail-Adresse unter Eltern. Informieren Sie die
            Familie am besten per Telefon.
          </span>
        </>
      }
    />
  );
}

/** The mark in the list: at least one mail did not reach the parents. */
export function HolidayRegistrationMailProblemBadge({
  problems,
}: Readonly<{ problems: readonly HolidayRegistrationMailProblem[] }>) {
  if (problems.length === 0) return null;
  return <StatusBadge tone="warning" label="E-Mail prüfen" />;
}
