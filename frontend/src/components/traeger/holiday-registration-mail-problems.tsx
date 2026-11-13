// Mails of a holiday registration that did not reach the parents (#6737):
// the receipt and the acceptance mail. The office learns it here, so it can
// check the address and reach the family another way.
import { useEffect, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { StatusBadge } from "~/components/ui/status-badge";
import { createLogger } from "~/lib/logger";
import {
  fetchHolidayRegistrationMailProblems,
  type HolidayRegistrationMailKind,
  type HolidayRegistrationMailProblem,
} from "~/lib/traeger-holiday-registrations-api";

const logger = createLogger({ component: "HolidayRegistrationMailProblems" });

type MailProblems = ReadonlyMap<
  string,
  readonly HolidayRegistrationMailProblem[]
>;
const NO_MAIL_PROBLEMS: MailProblems = new Map();

/**
 * The undelivered mails of the registrations of a period, by registration
 * ID. Empty while it loads and when the read fails. `reload` (for example the
 * IDs and states of the loaded registrations) asks again when it changes.
 */
export function useHolidayRegistrationMailProblems(
  periodId: string,
  reload: string,
): MailProblems {
  const [loaded, setLoaded] = useState<{
    key: string;
    problems: MailProblems;
  } | null>(null);
  const key = `${periodId}|${reload}`;
  useEffect(() => {
    if (!periodId || reload === "") return;
    let current = true;
    const load = async () => {
      let next = NO_MAIL_PROBLEMS;
      try {
        next = await fetchHolidayRegistrationMailProblems(periodId);
      } catch (err) {
        // Silent on purpose: the mark only adds to the list, which works
        // without it; each registration still shows its mails.
        logger.warn("holiday_mail_problems_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      }
      if (current) setLoaded({ key, problems: next });
    };
    void load();
    return () => {
      current = false;
    };
  }, [periodId, reload, key]);
  return loaded?.key === key ? loaded.problems : NO_MAIL_PROBLEMS;
}

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
      return `${Mail} wurde nicht gesendet. An diese Adresse gehen zurzeit keine Mails.`;
    case "unknown":
      return `Ob ${mail} angekommen ist, ist unklar.`;
    default:
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
