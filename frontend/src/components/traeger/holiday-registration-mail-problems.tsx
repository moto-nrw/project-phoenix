// Mails of a holiday registration that did not reach the parents (#6737,
// #6976): receipt, acceptance, place, closing day notice and parent account
// offer. The office learns it here, so it can check the address and reach
// the family another way.
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
// office's „Bestätigen“. Every name is feminine, so „die“ and „der“ fit.
export const HOLIDAY_MAIL_LABELS: Readonly<
  Record<HolidayRegistrationMailKind, string>
> = {
  holiday_registration_received: "Eingangsbestätigung",
  holiday_registration_accepted: "Annahme-Mail",
  holiday_place_granted: "Mail zum Ferienplatz",
  holiday_closure_notice: "Mail zu Schließtagen",
  holiday_parent_account_offer: "Mail zum Elternkonto",
};

/** The name of a mail in a list, as „Eingangsbestätigung“ (#6976). */
function holidayMailLabel(kind: HolidayRegistrationMailKind): string {
  return HOLIDAY_MAIL_LABELS[kind] ?? "E-Mail";
}

/** What the office does when a mail did not reach the parents. */
export const HOLIDAY_MAIL_NEXT_STEP =
  "Informieren Sie die Familie am besten per Telefon. Prüfen Sie die E-Mail-Adresse unter „Eltern“ und ändern Sie sie, wenn sie falsch ist.";

/** One sentence per mail: which mail and what became of it. */
function holidayMailProblemText(
  problem: HolidayRegistrationMailProblem,
): string {
  const mail = `die ${holidayMailLabel(problem.kind)}`;
  const Mail = mail.charAt(0).toUpperCase() + mail.slice(1);
  switch (problem.state) {
    case "blocked":
      return `${Mail} wurde nicht gesendet. An diese Adresse gehen zurzeit keine Mails.`;
    case "unknown":
      return `Ob ${mail} angekommen ist, ist unklar.`;
    // These mails arrived; the parents refused them afterwards (#6976).
    case "complained":
      return `Die Eltern haben ${mail} als unerwünscht gemeldet.`;
    case "unsubscribed":
      return `Nach der ${holidayMailLabel(problem.kind)} haben sich die Eltern von den Mails abgemeldet.`;
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
          {problems.map((problem, index) => (
            <span key={`${problem.kind}-${index}`} className="block">
              {holidayMailProblemText(problem)}
            </span>
          ))}
          <span className="block">{HOLIDAY_MAIL_NEXT_STEP}</span>
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
