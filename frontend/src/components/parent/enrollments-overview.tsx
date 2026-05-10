"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import {
  type EnrollmentChildStatus,
  type EnrollmentRequest,
  listMyEnrollments,
} from "~/lib/parent-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollmentsOverview" });

const CHILD_STATUS_LABEL: Record<EnrollmentChildStatus, string> = {
  submitted: "Eingereicht",
  under_review: "In Prüfung",
  approved: "Bestätigt",
  waitlisted: "Warteliste",
  rejected: "Abgelehnt",
  withdrawn: "Zurückgezogen",
};

const CHILD_STATUS_COLORS: Record<
  EnrollmentChildStatus,
  { bg: string; text: string }
> = {
  submitted: { bg: "#5080D8", text: "#FFFFFF" },
  under_review: { bg: "#5080D8", text: "#FFFFFF" },
  approved: { bg: "#83CD2D", text: "#FFFFFF" },
  waitlisted: { bg: "#F78C10", text: "#FFFFFF" },
  rejected: { bg: "#D6373E", text: "#FFFFFF" },
  withdrawn: { bg: "#6B7280", text: "#FFFFFF" },
};

function formatDate(iso: string | undefined): string {
  if (!iso) return "—";
  try {
    return new Date(iso).toLocaleDateString("de-DE", {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
    });
  } catch {
    return iso;
  }
}

interface RequestCardProps {
  readonly request: EnrollmentRequest;
}

function RequestCard({ request }: RequestCardProps) {
  return (
    <Link
      href={`/parents/enroll/status/${encodeURIComponent(request.status_token)}`}
      className="block rounded-2xl border border-gray-200 bg-white p-5 shadow-sm transition-colors hover:bg-gray-50"
    >
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <div>
          <h3 className="text-base font-semibold text-gray-900">
            {request.school_name}
          </h3>
          <p className="text-xs text-gray-600">
            {request.phase_name} · Betreuung{" "}
            {formatDate(request.service_start_date)} –{" "}
            {formatDate(request.service_end_date)}
          </p>
        </div>
        <p className="text-xs text-gray-500">
          Eingereicht am {formatDate(request.submitted_at)}
        </p>
      </div>
      <ul className="mt-3 space-y-2">
        {request.children.map((c) => {
          const palette =
            CHILD_STATUS_COLORS[c.status] ?? CHILD_STATUS_COLORS.submitted;
          return (
            <li
              key={c.child_id}
              className="flex items-center justify-between rounded-lg border border-gray-100 px-3 py-2"
            >
              <span className="text-sm text-gray-900">
                {c.first_name} {c.last_name}
              </span>
              <span
                className="rounded-full px-3 py-1 text-xs font-medium"
                style={{ backgroundColor: palette.bg, color: palette.text }}
              >
                {CHILD_STATUS_LABEL[c.status] ?? c.status}
              </span>
            </li>
          );
        })}
      </ul>
    </Link>
  );
}

/**
 * Lists every enrollment.requests row the calling parent owns,
 * rendered as one card per submission. Each card links to the
 * existing status page (/parents/enroll/status/{token}) where the
 * parent can view, edit, or withdraw the submission.
 *
 * Renders nothing when the parent has no requests so the dashboard
 * doesn't show an empty section for accounts that haven't submitted
 * anything yet.
 */
export function EnrollmentsOverview() {
  const [requests, setRequests] = useState<EnrollmentRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const list = await listMyEnrollments();
      setRequests(list);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("enrollments_load_failed", { error: message });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  if (loading) {
    // Quietly absent during the initial load — the children section
    // already shows its own loading state and we don't want two
    // spinners stacked.
    return null;
  }
  if (error) {
    return (
      <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-800">
        {error}
      </div>
    );
  }
  if (requests.length === 0) {
    return null;
  }

  return (
    <section className="space-y-3">
      <header>
        <h2 className="text-lg font-semibold text-gray-900">
          Meine Anmeldungen
        </h2>
        <p className="text-sm text-gray-600">
          {requests.length === 1
            ? "1 Anmeldung — Klicke darauf, um den Status einzusehen oder zu bearbeiten."
            : `${requests.length} Anmeldungen — Klicke darauf, um den Status einzusehen oder zu bearbeiten.`}
        </p>
      </header>
      <div className="space-y-3">
        {requests.map((r) => (
          <RequestCard key={r.request_id} request={r} />
        ))}
      </div>
    </section>
  );
}
