// HistoryLinks - extracted from student detail page
"use client";

import { InfoCard } from "~/components/ui/info-card";

/**
 * History links section with disabled buttons for room, feedback, and mensa history
 */
export function HistoryLinks() {
  return (
    <InfoCard title="Historien" icon={<ClockIcon />}>
      <div className="grid grid-cols-1 gap-2">
        <HistoryLinkButton
          icon={<BuildingIcon />}
          iconBgColor="bg-[#5080D8]"
          title="Raumverlauf"
          subtitle="Verlauf der Raumbesuche"
        />
        <HistoryLinkButton
          icon={<ChatIcon />}
          iconBgColor="bg-[#83CD2D]"
          title="Feedbackhistorie"
          subtitle="Feedback und Bewertungen"
        />
        <HistoryLinkButton
          icon={<ForkKnifeIcon />}
          iconBgColor="bg-[#F78C10]"
          title="Mensaverlauf"
          subtitle="Mahlzeiten und Bestellungen"
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
}

function HistoryLinkButton({
  icon,
  iconBgColor,
  title,
  subtitle,
}: HistoryLinkButtonProps) {
  return (
    <button
      type="button"
      disabled
      className="flex cursor-not-allowed items-center justify-between rounded-lg border border-gray-100 bg-gray-50 p-3 opacity-60"
    >
      <div className="flex items-center gap-3">
        <div
          className={`flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg ${iconBgColor} sm:h-9 sm:w-9`}
        >
          {icon}
        </div>
        <div className="min-w-0 flex-1 text-left">
          <p className="text-sm font-medium text-gray-400 sm:text-base">
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
        d="M8.5 3v18M7 3v3.5M10 3v3.5M7 10h3M15.5 3v3c0 1-2 2-2 2v13"
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
