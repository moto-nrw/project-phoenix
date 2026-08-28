import { downloadBlob, filenameFromDisposition } from "./file-download";
import { sessionFetch } from "./session-cache";

/**
 * Guardian payment data (#2608). Bank details hang off the guardian, not off
 * the child: a child has no bank account, and guardians are already shared
 * across siblings, so one maintained IBAN covers every sibling. Which of a
 * child's guardians is charged is a separate, per-child mark.
 *
 * Everything here needs the `guardians:financial` permission. The IBAN arrives
 * masked by default; `revealGuardianPayment` writes a GDPR access-log row
 * server-side, which is why it is a POST.
 */

export interface GuardianPaymentMasked {
  guardianId: string;
  ibanMasked: string | null;
  accountHolder: string | null;
}

export interface GuardianPaymentPlain {
  guardianId: string;
  iban: string | null;
  accountHolder: string | null;
}

export interface GuardianPaymentInput {
  iban: string | null;
  accountHolder: string | null;
}

export interface PaymentOverviewRow {
  studentId: string;
  studentName: string;
  schoolClass: string;
  guardianId: string | null;
  guardianName: string;
  relationshipType: string;
  accountHolder: string;
  ibanMasked: string;
}

export type PaymentExportFormat = "pdf" | "xlsx" | "docx";

interface BackendMasked {
  guardian_id: string;
  iban_masked: string | null;
  account_holder: string | null;
}

interface BackendPlain {
  guardian_id: string;
  iban: string | null;
  account_holder: string | null;
}

interface BackendOverviewRow {
  student_id: string;
  student_name: string;
  school_class: string;
  guardian_id: string | null;
  guardian_name: string;
  relationship_type: string;
  account_holder: string;
  iban_masked: string;
}

// The backend answers errors as {"status":"error","error":"..."}. Surfacing the
// inner string keeps German validation messages ("malformed IBAN") readable in
// the toast instead of a bare status code.
async function readError(
  response: Response,
  fallback: string,
): Promise<string> {
  try {
    const json = (await response.json()) as { error?: unknown };
    if (typeof json.error === "string" && json.error.trim()) return json.error;
  } catch {
    // Not JSON — fall through.
  }
  return fallback;
}

export async function fetchGuardianPayment(
  guardianId: string,
): Promise<GuardianPaymentMasked> {
  const response = await sessionFetch(`/api/guardians/${guardianId}/payment`);
  if (!response.ok) {
    throw new Error(
      await readError(
        response,
        "Die Bankverbindung konnte nicht geladen werden. Bitte noch einmal versuchen.",
      ),
    );
  }
  const json = (await response.json()) as { data: BackendMasked };
  return {
    guardianId: json.data.guardian_id,
    ibanMasked: json.data.iban_masked,
    accountHolder: json.data.account_holder,
  };
}

export async function revealGuardianPayment(
  guardianId: string,
): Promise<GuardianPaymentPlain> {
  const response = await sessionFetch(
    `/api/guardians/${guardianId}/payment/reveal`,
    { method: "POST", headers: { "Content-Type": "application/json" } },
  );
  if (!response.ok) {
    throw new Error(
      await readError(
        response,
        "Die IBAN konnte nicht angezeigt werden. Bitte noch einmal versuchen.",
      ),
    );
  }
  const json = (await response.json()) as { data: BackendPlain };
  return {
    guardianId: json.data.guardian_id,
    iban: json.data.iban,
    accountHolder: json.data.account_holder,
  };
}

export async function updateGuardianPayment(
  guardianId: string,
  input: GuardianPaymentInput,
): Promise<void> {
  const response = await sessionFetch(`/api/guardians/${guardianId}/payment`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      iban: input.iban,
      account_holder: input.accountHolder,
    }),
  });
  if (!response.ok) {
    throw new Error(
      await readError(
        response,
        "Die Bankverbindung konnte nicht gespeichert werden. Bitte noch einmal versuchen.",
      ),
    );
  }
}

/** Marks which guardian pays for this child; null clears the assignment. */
export async function setStudentPayer(
  studentId: string,
  guardianId: string | null,
): Promise<void> {
  const response = await sessionFetch(
    `/api/guardians/students/${studentId}/payer`,
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ guardian_id: guardianId }),
    },
  );
  if (!response.ok) {
    throw new Error(
      await readError(
        response,
        "Das Zahlungskonto konnte nicht gespeichert werden. Bitte noch einmal versuchen.",
      ),
    );
  }
}

export async function fetchPaymentOverview(): Promise<PaymentOverviewRow[]> {
  const response = await sessionFetch("/api/guardians/payment-overview");
  if (!response.ok) {
    throw new Error(
      await readError(
        response,
        "Die Liste konnte nicht geladen werden. Bitte noch einmal versuchen.",
      ),
    );
  }
  const json = (await response.json()) as { data: BackendOverviewRow[] | null };
  return (json.data ?? []).map((row) => ({
    studentId: row.student_id,
    studentName: row.student_name,
    schoolClass: row.school_class,
    guardianId: row.guardian_id,
    guardianName: row.guardian_name,
    relationshipType: row.relationship_type,
    accountHolder: row.account_holder,
    ibanMasked: row.iban_masked,
  }));
}

/**
 * Downloads the Bankverbindungen list. The file carries UNMASKED IBANs and the
 * request is audited as its own event server-side.
 */
export async function exportPaymentOverview(
  format: PaymentExportFormat,
): Promise<void> {
  const response = await sessionFetch(
    "/api/guardians/payment-overview/export",
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ format }),
    },
  );
  if (!response.ok) {
    throw new Error(
      await readError(
        response,
        "Der Export hat nicht geklappt. Bitte noch einmal versuchen.",
      ),
    );
  }
  downloadBlob(
    await response.blob(),
    filenameFromDisposition(response) ?? `Bankverbindungen.${format}`,
  );
}
