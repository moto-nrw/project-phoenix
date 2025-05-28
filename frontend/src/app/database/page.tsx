"use client";

import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import Link from "next/link";
import { ResponsiveLayout } from "~/components/dashboard";
import { Suspense, useState, useEffect } from "react";

// Icon component
const Icon: React.FC<{ path: string; className?: string }> = ({ path, className }) => (
  <svg
    className={className}
    fill="none"
    viewBox="0 0 24 24"
    stroke="currentColor"
    strokeWidth={2}
  >
    <path strokeLinecap="round" strokeLinejoin="round" d={path} />
  </svg>
);

// Base data sections configuration
const baseDataSections = [
  {
    id: "students",
    title: "Schüler",
    description: "Schülerdaten verwalten und bearbeiten",
    href: "/database/students",
    icon: "M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z",
    color: "from-[#5080D8] to-[#4070c8]",
  },
  {
    id: "teachers",
    title: "Lehrer",
    description: "Lehrerdaten und Zuordnungen verwalten",
    href: "/database/teachers",
    icon: "M12 14l9-5-9-5-9 5 9 5z M12 14l6.16-3.422a12.083 12.083 0 01.665 6.479A11.952 11.952 0 0012 20.055a11.952 11.952 0 00-6.824-2.998 12.078 12.078 0 01.665-6.479L12 14z M12 14l9-5-9-5-9 5 9 5zm0 0l6.16-3.422a12.083 12.083 0 01.665 6.479A11.952 11.952 0 0012 20.055a11.952 11.952 0 00-6.824-2.998 12.078 12.078 0 01.665-6.479L12 14zm-4 6v-7.5l4-2.222",
    color: "from-[#F78C10] to-[#e57a00]",
  },
  {
    id: "rooms",
    title: "Räume",
    description: "Räume und Ausstattung verwalten",
    href: "/database/rooms",
    icon: "M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4",
    color: "from-[#83CD2D] to-[#70b525]",
  },
  {
    id: "activities",
    title: "Aktivitäten",
    description: "Aktivitäten und Zeitpläne verwalten",
    href: "/database/activities",
    icon: "M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2",
    color: "from-[#FF3130] to-[#e02020]",
  },
  {
    id: "groups",
    title: "Gruppen",
    description: "Gruppen und Kombinationen verwalten",
    href: "/database/groups",
    icon: "M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z",
    color: "from-purple-500 to-purple-600",
  },
];

function DatabaseContent() {
  const { data: session, status } = useSession({ required: true });
  const [counts, setCounts] = useState<{
    students: number;
    teachers: number;
    rooms: number;
    activities: number;
    groups: number;
  }>({
    students: 0,
    teachers: 0,
    rooms: 0,
    activities: 0,
    groups: 0,
  });
  const [countsLoading, setCountsLoading] = useState(true);

  // Fetch real counts from the database
  useEffect(() => {
    const fetchCounts = async () => {
      try {
        const response = await fetch("/api/database/counts");
        if (response.ok) {
          const data = await response.json() as typeof counts;
          setCounts(data);
        }
      } catch (error) {
        console.error("Error fetching counts:", error);
      } finally {
        setCountsLoading(false);
      }
    };

    if (session?.user) {
      void fetchCounts();
    }
  }, [session]);

  if (status === "loading") {
    return (
      <div className="flex min-h-[50vh] items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
      </div>
    );
  }

  if (!session?.user) {
    redirect("/");
  }

  return (
    <>
      {/* Header */}
      <div className="mb-8">
        <h1 className="text-2xl md:text-3xl font-bold text-gray-900">Datenverwaltung</h1>
        <p className="mt-2 text-sm md:text-base text-gray-600">
          Wählen Sie einen Bereich aus, um Daten zu verwalten
        </p>
      </div>

      {/* Data Section Cards */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {baseDataSections.map((section) => {
          // Get the count for this section
          const count = counts[section.id as keyof typeof counts] ?? 0;
          const countText = countsLoading ? "Lade..." : `${count} ${count === 1 ? 'Eintrag' : 'Einträge'}`;
          
          return (
            <Link
              key={section.id}
              href={section.href}
              className="group relative overflow-hidden rounded-xl border border-gray-200 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-xl hover:scale-[1.02] hover:border-gray-300"
            >
              {/* Background gradient on hover */}
              <div className={`absolute inset-0 bg-gradient-to-br ${section.color} opacity-0 group-hover:opacity-5 transition-opacity duration-300`} />
              
              {/* Content */}
              <div className="relative">
                {/* Icon and Count */}
                <div className="flex items-start justify-between mb-4">
                  <div className={`rounded-lg bg-gradient-to-br ${section.color} p-3 text-white shadow-lg group-hover:shadow-xl transition-all duration-300`}>
                    <Icon path={section.icon} className="h-6 w-6" />
                  </div>
                  <span className="text-xs font-medium text-gray-500 bg-gray-100 px-2 py-1 rounded-full">
                    {countText}
                  </span>
                </div>
                
                {/* Title and Description */}
                <h3 className="text-lg font-semibold text-gray-900 mb-1 group-hover:text-gray-800">
                  {section.title}
                </h3>
                <p className="text-sm text-gray-600 line-clamp-2">
                  {section.description}
                </p>
                
                {/* Arrow indicator */}
                <div className="mt-4 flex items-center text-gray-400 group-hover:text-gray-600 transition-colors">
                  <span className="text-sm font-medium">Verwalten</span>
                  <Icon 
                    path="M9 5l7 7-7 7" 
                    className="ml-2 h-4 w-4 transition-transform duration-300 group-hover:translate-x-1" 
                  />
                </div>
              </div>
            </Link>
          );
        })}
      </div>

      {/* Info Section */}
      <div className="mt-8 rounded-lg border border-blue-200 bg-blue-50 p-4">
        <div className="flex">
          <div className="flex-shrink-0">
            <Icon 
              path="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" 
              className="h-5 w-5 text-blue-600" 
            />
          </div>
          <div className="ml-3">
            <h3 className="text-sm font-medium text-blue-800">Hinweis zur Datenverwaltung</h3>
            <div className="mt-1 text-sm text-blue-700">
              <p>Änderungen an den Daten werden sofort wirksam. Bitte gehen Sie sorgfältig vor und überprüfen Sie Ihre Eingaben.</p>
            </div>
          </div>
        </div>
      </div>
    </>
  );
}

export default function DatabasePage() {
  return (
    <ResponsiveLayout userName="User">
      <Suspense
        fallback={
          <div className="flex min-h-[50vh] items-center justify-center">
            <div className="h-8 w-8 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
          </div>
        }
      >
        <DatabaseContent />
      </Suspense>
    </ResponsiveLayout>
  );
}
