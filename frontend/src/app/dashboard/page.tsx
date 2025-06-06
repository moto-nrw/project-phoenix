// app/dashboard/page.tsx
"use client";

import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import Link from "next/link";

// Info Card Component with proper TypeScript types and responsive design
interface InfoCardProps {
  title: string;
  children: React.ReactNode;
  href?: string;
  className?: string;
}

const InfoCard: React.FC<InfoCardProps> = ({ title, children, href, className }) => (
  <div className={`rounded-lg border border-gray-100 bg-white p-4 md:p-6 shadow-md h-full ${className ?? ""}`}>
    <div className="mb-3 md:mb-4 flex items-start md:items-center justify-between flex-col md:flex-row gap-2 md:gap-0">
      <h3 className="text-base md:text-lg font-semibold">{title}</h3>
      {href && (
        <Link
          href={href}
          className="text-xs md:text-sm font-medium text-purple-600 hover:text-purple-800 self-end md:self-auto"
        >
          <span className="hidden md:inline">
            {title.includes("OGS") ? "Meine OGS-Gruppe →" : "Alle anzeigen →"}
          </span>
          <span className="md:hidden">
            {title.includes("OGS") ? "OGS →" : "Alle →"}
          </span>
        </Link>
      )}
    </div>
    {children}
  </div>
);

// Student Stats Component
const StudentStats = () => (
  <InfoCard title="Schülerübersicht" href="/students/search">
    <div className="grid grid-cols-2 gap-2 md:gap-4">
      <div className="rounded-lg bg-blue-50 p-2 md:p-3">
        <p className="text-xs md:text-sm text-blue-800">Anwesend heute</p>
        <p className="text-xl md:text-2xl font-bold text-blue-900">127</p>
      </div>
      <div className="rounded-lg bg-green-50 p-2 md:p-3">
        <p className="text-xs md:text-sm text-green-800">Gesamt eingeschrieben</p>
        <p className="text-xl md:text-2xl font-bold text-green-900">150</p>
      </div>
      <div className="rounded-lg bg-amber-50 p-2 md:p-3">
        <p className="text-xs md:text-sm text-amber-800">Schulhof</p>
        <p className="text-xl md:text-2xl font-bold text-amber-900">32</p>
      </div>
      <div className="rounded-lg bg-purple-50 p-2 md:p-3">
        <p className="text-xs md:text-sm text-purple-800">Unterwegs</p>
        <p className="text-xl md:text-2xl font-bold text-purple-900">8</p>
      </div>
    </div>
    <div className="mt-3 md:mt-4">
      <h4 className="mb-2 text-xs md:text-sm font-medium text-gray-700">
        Zuletzt eingecheckt
      </h4>
      <ul className="divide-y divide-gray-200">
        <li className="py-1.5 md:py-2">
          <div className="flex justify-between items-center">
            <span className="text-xs md:text-sm text-gray-900 truncate pr-2">Max Mustermann (4a)</span>
            <span className="text-xs text-gray-500 flex-shrink-0">vor 5 min</span>
          </div>
        </li>
        <li className="py-1.5 md:py-2">
          <div className="flex justify-between items-center">
            <span className="text-xs md:text-sm text-gray-900 truncate pr-2">Emma Schmidt (3b)</span>
            <span className="text-xs text-gray-500 flex-shrink-0">vor 12 min</span>
          </div>
        </li>
        <li className="py-1.5 md:py-2">
          <div className="flex justify-between items-center">
            <span className="text-xs md:text-sm text-gray-900 truncate pr-2">Leon Wagner (5c)</span>
            <span className="text-xs text-gray-500 flex-shrink-0">vor 18 min</span>
          </div>
        </li>
      </ul>
    </div>
  </InfoCard>
);

// Activity Stats Component
const ActivityStats = () => (
  <InfoCard title="Aktivitäten und Räume" href="/database/activities">
    <div className="grid grid-cols-2 gap-2 md:gap-4">
      <div className="rounded-lg bg-purple-50 p-2 md:p-3">
        <p className="text-xs md:text-sm text-purple-800">Aktuelle Aktivitäten</p>
        <p className="text-xl md:text-2xl font-bold text-purple-900">15</p>
      </div>
      <div className="rounded-lg bg-green-50 p-2 md:p-3">
        <p className="text-xs md:text-sm text-green-800">Freie Räume</p>
        <p className="text-xl md:text-2xl font-bold text-green-900">8</p>
      </div>
      <div className="rounded-lg bg-blue-50 p-2 md:p-3">
        <p className="text-xs md:text-sm text-blue-800">Kapazität genutzt</p>
        <p className="text-xl md:text-2xl font-bold text-blue-900">73%</p>
      </div>
      <div className="rounded-lg bg-amber-50 p-2 md:p-3">
        <p className="text-xs md:text-sm text-amber-800">Kategorien</p>
        <p className="text-xl md:text-2xl font-bold text-amber-900">6</p>
      </div>
    </div>
    <div className="mt-3 md:mt-4">
      <h4 className="mb-2 text-xs md:text-sm font-medium text-gray-700">
        Aktuelle Aktivitäten
      </h4>
      <ul className="divide-y divide-gray-200">
        <li className="py-1.5 md:py-2">
          <div className="flex justify-between items-center">
            <div className="min-w-0 flex-1 pr-2">
              <p className="text-xs md:text-sm font-medium text-gray-900 truncate">Fußball AG</p>
              <p className="text-xs text-gray-500 truncate">Sport • 12/15 Teilnehmer</p>
            </div>
            <div className="h-2 w-2 flex-shrink-0 rounded-full bg-green-500"></div>
          </div>
        </li>
        <li className="py-1.5 md:py-2">
          <div className="flex justify-between items-center">
            <div className="min-w-0 flex-1 pr-2">
              <p className="text-xs md:text-sm font-medium text-gray-900 truncate">Coding Club</p>
              <p className="text-xs text-gray-500 truncate">Technik • 8/10 Teilnehmer</p>
            </div>
            <div className="h-2 w-2 flex-shrink-0 rounded-full bg-green-500"></div>
          </div>
        </li>
      </ul>
    </div>
  </InfoCard>
);

// OGS Groups Stats Component
const OGSGroupsStats = () => (
  <InfoCard title="OGS-Gruppen Übersicht" href="/ogs_groups">
    <div className="grid grid-cols-2 gap-2 md:gap-4">
      <div className="rounded-lg bg-amber-50 p-2 md:p-3">
        <p className="text-xs md:text-sm text-amber-800">Aktive Gruppen</p>
        <p className="text-xl md:text-2xl font-bold text-amber-900">8</p>
      </div>
      <div className="rounded-lg bg-purple-50 p-2 md:p-3">
        <p className="text-xs md:text-sm text-purple-800">In Gruppenräumen</p>
        <p className="text-xl md:text-2xl font-bold text-purple-900">35</p>
      </div>
      <div className="rounded-lg bg-blue-50 p-2 md:p-3">
        <p className="text-xs md:text-sm text-blue-800">Betreuer heute</p>
        <p className="text-xl md:text-2xl font-bold text-blue-900">14</p>
      </div>
      <div className="rounded-lg bg-green-50 p-2 md:p-3">
        <p className="text-xs md:text-sm text-green-800">In Heimatraum</p>
        <p className="text-xl md:text-2xl font-bold text-green-900">19</p>
      </div>
    </div>
    <div className="mt-3 md:mt-4">
      <h4 className="mb-2 text-xs md:text-sm font-medium text-gray-700">
        Letzte Gruppenaktivitäten
      </h4>
      <ul className="divide-y divide-gray-200">
        <li className="py-1.5 md:py-2">
          <div className="flex justify-between items-center">
            <div className="min-w-0 flex-1 pr-2">
              <p className="text-xs md:text-sm font-medium text-gray-900 truncate">Sonnenschein</p>
              <p className="text-xs text-gray-500 truncate">Klasse 1-2 • 24 Kinder</p>
            </div>
            <div className="h-2 w-2 flex-shrink-0 rounded-full bg-green-500"></div>
          </div>
        </li>
        <li className="py-1.5 md:py-2">
          <div className="flex justify-between items-center">
            <div className="min-w-0 flex-1 pr-2">
              <p className="text-xs md:text-sm font-medium text-gray-900 truncate">Regenbogen</p>
              <p className="text-xs text-gray-500 truncate">Klasse 2-3 • 22 Kinder</p>
            </div>
            <div className="h-2 w-2 flex-shrink-0 rounded-full bg-green-500"></div>
          </div>
        </li>
      </ul>
    </div>
  </InfoCard>
);

// Quick Actions Component
const QuickActions = () => (
  <InfoCard title="Schnellzugriff">
    <div className="grid grid-cols-1 gap-2 md:gap-3">
      <Link
        href="/students/search"
        className="flex items-center rounded-lg border border-gray-200 p-2 md:p-3 transition-all hover:border-blue-500 hover:bg-blue-50 active:bg-blue-100"
      >
        <div className="mr-2 md:mr-3 p-1.5 md:p-2 rounded-lg bg-blue-100">
          <svg className="h-4 w-4 md:h-5 md:w-5 text-blue-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
        </div>
        <div className="min-w-0 flex-1">
          <h4 className="text-sm md:text-base font-medium text-gray-900">Schüler finden</h4>
          <p className="text-xs text-gray-500 truncate">Schnellsuche nach Namen oder Klasse</p>
        </div>
      </Link>

      <Link
        href="/ogs_groups"
        className="flex items-center rounded-lg border border-gray-200 p-2 md:p-3 transition-all hover:border-green-500 hover:bg-green-50 active:bg-green-100"
      >
        <div className="mr-2 md:mr-3 p-1.5 md:p-2 rounded-lg bg-green-100">
          <svg className="h-4 w-4 md:h-5 md:w-5 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
          </svg>
        </div>
        <div className="min-w-0 flex-1">
          <h4 className="text-sm md:text-base font-medium text-gray-900">OGS Gruppe</h4>
          <p className="text-xs text-gray-500 truncate">Informationen meiner Gruppe</p>
        </div>
      </Link>

      <Link
        href="/statistics"
        className="flex items-center rounded-lg border border-gray-200 p-2 md:p-3 transition-all hover:border-amber-500 hover:bg-amber-50 active:bg-amber-100"
      >
        <div className="mr-2 md:mr-3 p-1.5 md:p-2 rounded-lg bg-amber-100">
          <svg className="h-4 w-4 md:h-5 md:w-5 text-amber-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
          </svg>
        </div>
        <div className="min-w-0 flex-1">
          <h4 className="text-sm md:text-base font-medium text-gray-900">Statistiken</h4>
          <p className="text-xs text-gray-500 truncate">Schulweite Kennzahlen und Daten</p>
        </div>
      </Link>

      <Link
        href="/substitutions"
        className="flex items-center rounded-lg border border-gray-200 p-2 md:p-3 transition-all hover:border-purple-500 hover:bg-purple-50 active:bg-purple-100"
      >
        <div className="mr-2 md:mr-3 p-1.5 md:p-2 rounded-lg bg-purple-100">
          <svg className="h-4 w-4 md:h-5 md:w-5 text-purple-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
          </svg>
        </div>
        <div className="min-w-0 flex-1">
          <h4 className="text-sm md:text-base font-medium text-gray-900">Vertretungen</h4>
          <p className="text-xs text-gray-500 truncate">Personalausfälle verwalten</p>
        </div>
      </Link>
    </div>
  </InfoCard>
);

// Helper function to get time-based greeting
function getTimeBasedGreeting(): string {
  const hour = new Date().getHours();
  if (hour < 12) return "Guten Morgen";
  if (hour < 17) return "Guten Tag";
  return "Guten Abend";
}

// Helper function to get current date in German format
function getCurrentDate(): string {
  const today = new Date();
  const options: Intl.DateTimeFormatOptions = {
    weekday: 'long',
    year: 'numeric',
    month: 'long',
    day: 'numeric'
  };
  return today.toLocaleDateString('de-DE', options);
}

export default function DashboardPage() {
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  if (status === "loading") {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p>Loading...</p>
      </div>
    );
  }


  const firstName = session?.user?.name?.split(' ')[0] ?? "Benutzer";
  const greeting = getTimeBasedGreeting();
  const currentDate = getCurrentDate();


  return (
    <ResponsiveLayout>
      <div className="max-w-7xl mx-auto">
        {/* Welcome Header with Time Context - Mobile Responsive */}
        <div className="mb-6 md:mb-8">
          <h1 className="text-2xl md:text-3xl font-bold text-gray-900 mb-1 md:mb-2">
            {greeting}, {firstName}!
          </h1>
          <div className="text-gray-600 text-base md:text-lg">
            {/* Mobile: Split date and description into separate lines */}
            <div className="block md:hidden">
              <div className="text-sm">{currentDate}</div>
              <div className="text-sm mt-1">Deine Übersicht für heute</div>
            </div>
            {/* Desktop: Single line with bullet */}
            <div className="hidden md:block">
              {currentDate} • Hier ist deine Übersicht für heute
            </div>
          </div>
        </div>

        {/* Stats Grid - Mobile Responsive with Equal Heights */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 md:gap-6 mb-6 md:mb-8">
          <div className="flex flex-col gap-4 md:gap-6">
            <QuickActions />
            <StudentStats />
          </div>
          <div className="flex flex-col gap-4 md:gap-6">
            <OGSGroupsStats />
            <ActivityStats />
          </div>
        </div>
      </div>
    </ResponsiveLayout>
  );
}
