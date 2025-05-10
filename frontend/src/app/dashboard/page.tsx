// app/dashboard/page.tsx
"use client";

import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { Header } from "~/components/dashboard/header";
import { Sidebar } from "~/components/dashboard/sidebar";
import Link from "next/link";

// Info Card Component with proper TypeScript types
interface InfoCardProps {
    title: string;
    children: React.ReactNode;
    href?: string;
}

const InfoCard: React.FC<InfoCardProps> = ({ title, children, href }) => (
    <div className="rounded-lg border border-gray-100 bg-white p-6 shadow-sm">
        <div className="mb-4 flex items-center justify-between">
            <h3 className="text-lg font-semibold">{title}</h3>
            {href && (
                <Link
                    href={href}
                    className="text-sm font-medium text-purple-600 hover:text-purple-800"
                >
                    Alle anzeigen →
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
            <div className="rounded-lg bg-yellow-50 p-3">
                <p className="text-sm text-yellow-800">Schulhof</p>
                <p className="text-2xl font-bold text-yellow-900">32</p>
            </div>
            <div className="rounded-lg bg-purple-50 p-3">
                <p className="text-sm text-purple-800">Toilette</p>
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

// Room Stats Component
const RoomStats = () => (
    <InfoCard title="Raumübersicht" href="/rooms">
        <div className="grid grid-cols-2 gap-4">
            <div className="rounded-lg bg-green-50 p-3">
                <p className="text-sm text-green-800">Belegte Räume</p>
                <p className="text-2xl font-bold text-green-900">12</p>
            </div>
            <div className="rounded-lg bg-blue-50 p-3">
                <p className="text-sm text-blue-800">Verfügbar</p>
                <p className="text-2xl font-bold text-blue-900">6</p>
            </div>
            <div className="rounded-lg bg-orange-50 p-3">
                <p className="text-sm text-orange-800">Auslastung</p>
                <p className="text-2xl font-bold text-orange-900">67%</p>
            </div>
            <div className="rounded-lg bg-indigo-50 p-3">
                <p className="text-sm text-indigo-800">Reserviert</p>
                <p className="text-2xl font-bold text-indigo-900">3</p>
            </div>
        </div>
        <div className="mt-4">
            <h4 className="mb-2 text-sm font-medium text-gray-700">
                Aktuelle Belegung
            </h4>
            <ul className="space-y-2">
                <li className="flex items-center text-sm">
                    <div className="mr-2 h-3 w-3 rounded bg-red-500"></div>
                    <div className="flex-1">Mensa - Mittagessen (Klasse 3-4)</div>
                    <span className="text-xs text-gray-500">12:00-13:00</span>
                </li>
                <li className="flex items-center text-sm">
                    <div className="mr-2 h-3 w-3 rounded bg-blue-500"></div>
                    <div className="flex-1">Raum 102 - Mathe AG</div>
                    <span className="text-xs text-gray-500">13:30-14:30</span>
                </li>
                <li className="flex items-center text-sm">
                    <div className="mr-2 h-3 w-3 rounded bg-green-500"></div>
                    <div className="flex-1">Sporthalle - Fußball Training</div>
                    <span className="text-xs text-gray-500">14:00-15:30</span>
                </li>
            </ul>
        </div>
    </InfoCard>
);

// Activity Stats Component
const ActivityStats = () => (
    <InfoCard title="Aktivitätsübersicht" href="/database/activities">
        <div className="grid grid-cols-2 gap-4">
            <div className="rounded-lg bg-purple-50 p-3">
                <p className="text-sm text-purple-800">Aktivitäten gesamt</p>
                <p className="text-2xl font-bold text-purple-900">15</p>
            </div>
            <div className="rounded-lg bg-green-50 p-3">
                <p className="text-sm text-green-800">Offen für Anmeldung</p>
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

// Quick Actions Component
const QuickActions = () => (
    <InfoCard title="Schnellzugriff">
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
                href="/import"
                className="flex items-center rounded-lg border border-gray-200 p-3 transition-all hover:border-green-500 hover:bg-green-50"
            >
                <div className="mr-3 p-2 rounded-lg bg-green-100">
                    <svg className="h-5 w-5 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M9 19l3 3m0 0l3-3m-3 3V10" />
                    </svg>
                </div>
                <div>
                    <h4 className="font-medium text-gray-900">Daten importieren</h4>
                    <p className="text-xs text-gray-500">CSV-Dateien importieren</p>
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
            redirect("/login");
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
        <div className="min-h-screen bg-gray-50">
            {/* Header */}
            <Header userName={session?.user?.name ?? "Root"} />

            <div className="flex">
                {/* Sidebar Navigation - Jetzt als separate Komponente */}
                <Sidebar />

                {/* Main Content */}
                <main className="flex-1 p-8">
                    <div className="max-w-7xl mx-auto">
                        <h1 className="text-2xl font-bold text-gray-900 mb-8">Dashboard</h1>

                        {/* Stats Grid */}
                        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
                            <div className="grid grid-cols-1 gap-6">
                                <StudentStats />
                                <RoomStats />
                            </div>
                            <div className="grid grid-cols-1 gap-6">
                                <ActivityStats />
                                <QuickActions />
                            </div>
                        </div>
                    </div>
                </main>
            </div>
        </div>
    );
}