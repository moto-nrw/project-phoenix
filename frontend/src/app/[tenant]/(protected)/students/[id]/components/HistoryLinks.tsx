// HistoryLinks - extracted from student detail page
"use client";

import { InfoCard } from "~/components/ui/info-card";
import { useTenantRouter } from "~/lib/tenant-router";

interface HistoryLinksProps {
  readonly studentId: string;
  /**
   * When true, the Raumverlauf (attendance history) button is active and
   * navigates to the attendance log page. Should mirror the tenant's
   * `gdpr.attendance_log_enabled` setting.
   */
  readonly attendanceLogEnabled: boolean;
  /**
   * When true, the Feedbackhistorie button is active and navigates to the
   * feedback history page. Should mirror the tenant's `feedback.enabled` setting.
   */
  readonly feedbackEnabled: boolean;
}

/**
 * History links section. The Raumverlauf button is gated by the tenant's
 * GDPR setting; the Feedbackhistorie button is gated by the tenant's
 * feedback.enabled setting; mensa history remains a disabled placeholder.
 */
export function HistoryLinks({
  studentId,
  attendanceLogEnabled,
  feedbackEnabled,
}: HistoryLinksProps) {
  const router = useTenantRouter();

  return (
    <InfoCard title="Historien" icon={<ClockIcon />}>
      <div className="grid grid-cols-1 gap-2">
        <HistoryLinkButton
          icon={<BuildingIcon />}
          iconBgColor="bg-[#5080D8]"
          title="Anwesenheitsprotokoll"
          subtitle={
            attendanceLogEnabled
              ? "Anwesenheit und besuchte Räume"
              : "Für Ihre Schule deaktiviert"
          }
          disabled={!attendanceLogEnabled}
          onClick={
            attendanceLogEnabled
              ? () => router.push(`/students/${studentId}/room-history`)
              : undefined
          }
        />
        <HistoryLinkButton
          icon={<ChatIcon />}
          iconBgColor="bg-[#83CD2D]"
          title="Feedbackhistorie"
          subtitle={
            feedbackEnabled
              ? "Feedback und Bewertungen"
              : "Für Ihre Schule deaktiviert"
          }
          disabled={!feedbackEnabled}
          onClick={
            feedbackEnabled
              ? () => router.push(`/students/${studentId}/feedback_history`)
              : undefined
          }
        />
        <HistoryLinkButton
          icon={<ForkKnifeIcon />}
          iconBgColor="bg-[#F78C10]"
          title="Mensaverlauf"
          subtitle="Mahlzeiten und Bestellungen"
          disabled
        />
      </div>
    </InfoCard>
  );
}

interface HistoryLinkButtonProps {
  readonly icon: React.ReactNode;
  readonly iconBgColor: string;
  readonly title: string;
  readonly subtitle: string;
  readonly disabled?: boolean;
  readonly onClick?: () => void;
}

function HistoryLinkButton({
  icon,
  iconBgColor,
  title,
  subtitle,
  disabled = false,
  onClick,
}: HistoryLinkButtonProps) {
  const baseClasses = "flex items-center justify-between rounded-lg border p-3";
  const stateClasses = disabled
    ? "cursor-not-allowed border-gray-100 bg-gray-50 opacity-60"
    : "cursor-pointer border-gray-200 bg-white transition-colors hover:bg-gray-50";

  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      className={`${baseClasses} ${stateClasses}`}
    >
      <div className="flex items-center gap-3">
        <div
          className={`flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg ${iconBgColor} sm:h-9 sm:w-9`}
        >
          {icon}
        </div>
        <div className="min-w-0 flex-1 text-left">
          <p
            className={`text-sm font-medium sm:text-base ${
              disabled ? "text-gray-400" : "text-gray-900"
            }`}
          >
            {title}
          </p>
          <p className="text-xs text-gray-400">{subtitle}</p>
        </div>
      </div>
      <ChevronRightIcon />
    </button>
  );
}

// SVG Icons
function ClockIcon() {
  return (
    <svg
      className="h-5 w-5"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
      />
    </svg>
  );
}

function BuildingIcon() {
  return (
    <svg
      className="h-4 w-4 text-white"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
      />
    </svg>
  );
}

function ChatIcon() {
  return (
    <svg
      className="h-4 w-4 text-white"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M7 8h10M7 12h4m1 8l-4-4H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-3l-4 4z"
      />
    </svg>
  );
}

function ForkKnifeIcon() {
  return (
    <svg
      className="h-4 w-4 text-white"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M17 21a1 1 0 0 0 1-1v-5.35c0-.457.316-.844.727-1.041a4 4 0 0 0-2.134-7.589 5 5 0 0 0-9.186 0 4 4 0 0 0-2.134 7.588c.411.198.727.585.727 1.041V20a1 1 0 0 0 1 1ZM6 17h12"
      />
    </svg>
  );
}

function ChevronRightIcon() {
  return (
    <svg
      className="h-4 w-4 flex-shrink-0 text-gray-300"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M9 5l7 7-7 7"
      />
    </svg>
  );
}
