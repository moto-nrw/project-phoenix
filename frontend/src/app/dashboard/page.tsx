// app/dashboard/page.tsx
"use client";

import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import Link from "next/link";

// Info Card Component with proper TypeScript types and fixed height option
interface InfoCardProps {
    title: string;
    children: React.ReactNode;
    href?: string;
    className?: string;
    fixedHeight?: boolean;
}

const InfoCard: React.FC<InfoCardProps> = ({ title, children, href, className, fixedHeight }) => (
    <div className={`rounded-lg border border-gray-100 bg-white p-6 shadow-md ${fixedHeight ? 'h-[420px]' : 'h-full'} ${className ?? ""}`}>
        <div className="mb-4 flex items-center justify-between">
            <h3 className="text-lg font-semibold">{title}</h3>
            {href && (
                <Link
                    href={href}
                    className="text-sm font-medium text-purple-600 hover:text-purple-800"
                >
                    {title.includes("OGS") ? "Meine OGS-Gruppe →" : "Alle anzeigen →"}
                </Link>
            )}
        </div>
        {children}
    </div>
);

// Student Stats Component
const StudentStats = () => (
    <InfoCard title="Schülerübersicht" href="/students/search">
        <div className="grid grid-cols-2 gap-4">
            <div className="rounded-lg bg-blue-50 p-3">
                <p className="text-sm text-blue-800">Anwesend heute</p>
                <p className="text-2xl font-bold text-blue-900">127</p>
            </div>
            <div className="rounded-lg bg-green-50 p-3">
                <p className="text-sm text-green-800">Gesamt eingeschrieben</p>
                <p className="text-2xl font-bold text-green-900">150</p>
            </div>
            <div className="rounded-lg bg-amber-50 p-3">
                <p className="text-sm text-amber-800">Schulhof</p>
                <p className="text-2xl font-bold text-amber-900">32</p>
            </div>
            <div className="rounded-lg bg-purple-50 p-3">
                <p className="text-sm text-purple-800">Unterwegs</p>
                <p className="text-2xl font-bold text-purple-900">8</p>
            </div>
        </div>
        <div className="mt-4">
            <h4 className="mb-2 text-sm font-medium text-gray-700">
                Zuletzt eingecheckt
            </h4>
            <ul className="divide-y divide-gray-200">
                <li className="py-2">
                    <div className="flex justify-between">
                        <span className="text-sm text-gray-900">Max Mustermann (4a)</span>
                        <span className="text-xs text-gray-500">vor 5 min</span>
                    </div>
                </li>
                <li className="py-2">
                    <div className="flex justify-between">
                        <span className="text-sm text-gray-900">Emma Schmidt (3b)</span>
                        <span className="text-xs text-gray-500">vor 12 min</span>
                    </div>
                </li>
                <li className="py-2">
                    <div className="flex justify-between">
                        <span className="text-sm text-gray-900">Leon Wagner (5c)</span>
                        <span className="text-xs text-gray-500">vor 18 min</span>
                    </div>
                </li>
            </ul>
        </div>
    </InfoCard>
);

// Activity Stats Component
const ActivityStats = () => (
    <InfoCard title="Aktivitäten und Räume" href="/database/activities">
        <div className="grid grid-cols-2 gap-4">
            <div className="rounded-lg bg-purple-50 p-3">
                <p className="text-sm text-purple-800">Aktuelle Aktivitäten</p>
                <p className="text-2xl font-bold text-purple-900">15</p>
            </div>
            <div className="rounded-lg bg-green-50 p-3">
                <p className="text-sm text-green-800">Freie Räume</p>
                <p className="text-2xl font-bold text-green-900">8</p>
            </div>
            <div className="rounded-lg bg-blue-50 p-3">
                <p className="text-sm text-blue-800">Kapazität genutzt</p>
                <p className="text-2xl font-bold text-blue-900">73%</p>
            </div>
            <div className="rounded-lg bg-amber-50 p-3">
                <p className="text-sm text-amber-800">Kategorien</p>
                <p className="text-2xl font-bold text-amber-900">6</p>
            </div>
        </div>
        <div className="mt-4">
            <h4 className="mb-2 text-sm font-medium text-gray-700">
                Aktuelle Aktivitäten
            </h4>
            <ul className="divide-y divide-gray-200">
                <li className="py-2">
                    <div className="flex justify-between">
                        <div>
                            <p className="text-sm font-medium text-gray-900">Fußball AG</p>
                            <p className="text-xs text-gray-500">Sport • 12/15 Teilnehmer</p>
                        </div>
                        <div className="h-2 w-2 self-center rounded-full bg-green-500"></div>
                    </div>
                </li>
                <li className="py-2">
                    <div className="flex justify-between">
                        <div>
                            <p className="text-sm font-medium text-gray-900">Coding Club</p>
                            <p className="text-xs text-gray-500">Technik • 8/10 Teilnehmer</p>
                        </div>
                        <div className="h-2 w-2 self-center rounded-full bg-green-500"></div>
                    </div>
                </li>
            </ul>
        </div>
    </InfoCard>
);

// OGS Groups Stats Component
const OGSGroupsStats = () => (
    <InfoCard title="OGS-Gruppen Übersicht" href="/ogs-groups" fixedHeight={true}>
        <div className="grid grid-cols-2 gap-4">
            <div className="rounded-lg bg-amber-50 p-3">
                <p className="text-sm text-amber-800">Aktive Gruppen</p>
                <p className="text-2xl font-bold text-amber-900">8</p>
            </div>
            <div className="rounded-lg bg-purple-50 p-3">
                <p className="text-sm text-purple-800">In Gruppenräumen</p>
                <p className="text-2xl font-bold text-purple-900">35</p>
            </div>
            <div className="rounded-lg bg-blue-50 p-3">
                <p className="text-sm text-blue-800">Betreuer heute</p>
                <p className="text-2xl font-bold text-blue-900">14</p>
            </div>
            <div className="rounded-lg bg-green-50 p-3">
                <p className="text-sm text-green-800">In Heimatraum</p>
                <p className="text-2xl font-bold text-green-900">19</p>
            </div>
        </div>
        <div className="mt-4">
            <h4 className="mb-2 text-sm font-medium text-gray-700">
                Letzte Gruppenaktivitäten
            </h4>
            <ul className="divide-y divide-gray-200">
                <li className="py-2">
                    <div className="flex justify-between">
                        <div>
                            <p className="text-sm font-medium text-gray-900">Sonnenschein</p>
                            <p className="text-xs text-gray-500">Klasse 1-2 • 24 Kinder</p>
                        </div>
                        <div className="h-2 w-2 self-center rounded-full bg-green-500"></div>
                    </div>
                </li>
                <li className="py-2">
                    <div className="flex justify-between">
                        <div>
                            <p className="text-sm font-medium text-gray-900">Regenbogen</p>
                            <p className="text-xs text-gray-500">Klasse 2-3 • 22 Kinder</p>
                        </div>
                        <div className="h-2 w-2 self-center rounded-full bg-green-500"></div>
                    </div>
                </li>
            </ul>
        </div>
    </InfoCard>
);

// Quick Actions Component
const QuickActions = () => (
    <InfoCard title="Schnellzugriff" fixedHeight={true}>
        <div className="grid grid-cols-1 gap-3">
            <Link
                href="/students/search"
                className="flex items-center rounded-lg border border-gray-200 p-3 transition-all hover:border-blue-500 hover:bg-blue-50"
            >
                <div className="mr-3 p-2 rounded-lg bg-blue-100">
                    <svg className="h-5 w-5 text-blue-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                    </svg>
                </div>
                <div>
                    <h4 className="font-medium text-gray-900">Schüler finden</h4>
                    <p className="text-xs text-gray-500">Schnellsuche nach Namen oder Klasse</p>
                </div>
            </Link>

            <Link
                href="/ogs-groups"
                className="flex items-center rounded-lg border border-gray-200 p-3 transition-all hover:border-green-500 hover:bg-green-50"
            >
                <div className="mr-3 p-2 rounded-lg bg-green-100">
                    <svg className="h-5 w-5 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
                    </svg>
                </div>
                <div>
                    <h4 className="font-medium text-gray-900">OGS Gruppe</h4>
                    <p className="text-xs text-gray-500">Informationen meiner Gruppe</p>
                </div>
            </Link>

            <Link
                href="/statistics"
                className="flex items-center rounded-lg border border-gray-200 p-3 transition-all hover:border-amber-500 hover:bg-amber-50"
            >
                <div className="mr-3 p-2 rounded-lg bg-amber-100">
                    <svg className="h-5 w-5 text-amber-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
                    </svg>
                </div>
                <div>
                    <h4 className="font-medium text-gray-900">Aktuelle Statistiken einsehen</h4>
                    <p className="text-xs text-gray-500">Schulweite Kennzahlen und Daten</p>
                </div>
            </Link>

            <Link
                href="/substitutions"
                className="flex items-center rounded-lg border border-gray-200 p-3 transition-all hover:border-purple-500 hover:bg-purple-50"
            >
                <div className="mr-3 p-2 rounded-lg bg-purple-100">
                    <svg className="h-5 w-5 text-purple-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
                    </svg>
                </div>
                <div>
                    <h4 className="font-medium text-gray-900">Vertretungen planen</h4>
                    <p className="text-xs text-gray-500">Personalausfälle verwalten</p>
                </div>
            </Link>
        </div>
    </InfoCard>
);

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

    return (
        <ResponsiveLayout userName={session?.user?.name ?? "Root"}>
            <div className="max-w-7xl mx-auto">
                <h1 className="text-4xl font-bold text-gray-900 mb-8">Home</h1>

                {/* Stats Grid */}
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
                    <div className="grid grid-cols-1 gap-6">
                        <QuickActions />
                        <StudentStats />
                    </div>
                    <div className="grid grid-cols-1 gap-6">
                        <OGSGroupsStats />
                        <ActivityStats />
                    </div>
                </div>
            </div>
        </ResponsiveLayout>
    );
}