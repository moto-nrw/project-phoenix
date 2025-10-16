import type { Teacher } from "@/lib/teacher-api";
import Link from "next/link";

interface TeacherListProps {
    teachers: Teacher[];
    onTeacherClick?: (teacher: Teacher) => void;
    emptyMessage?: string;
}

export default function TeacherList({
                                        teachers,
                                        onTeacherClick,
                                        emptyMessage = "Keine Lehrer vorhanden.",
                                    }: TeacherListProps) {
    if (teachers.length === 0) {
        return (
            <div className="flex flex-col items-center justify-center rounded-lg border border-gray-100 bg-white p-12 shadow-sm">
                <svg
                    xmlns="http://www.w3.org/2000/svg"
                    className="mb-4 h-16 w-16 text-gray-300"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                >
                    <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={1.5}
                        d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
                    />
                </svg>
                <p className="text-lg font-medium text-gray-600">{emptyMessage}</p>
                <p className="mt-2 text-sm text-gray-500">
                    Fügen Sie einen neuen Lehrer hinzu, um zu beginnen.
                </p>
                <Link href="/database/teachers/new">
                    <button className="mt-4 rounded-lg bg-blue-50 px-4 py-2 text-blue-600 transition-colors hover:bg-blue-100">
                        Lehrer erstellen
                    </button>
                </Link>
            </div>
        );
    }

    return (
        <div className="space-y-4 rounded-lg border border-gray-100 bg-white p-6 shadow-sm">
            {teachers.map((teacher) => (
                <div
                    key={teacher.id}
                    onClick={() => onTeacherClick?.(teacher)}
                    className={`group rounded-lg border border-gray-100 p-4 transition-all hover:border-blue-200 hover:bg-blue-50 ${
                        onTeacherClick ? "cursor-pointer" : ""
                    }`}
                >
                    <div className="flex items-center justify-between">
                        <div className="flex items-center space-x-4">
                            {/* Avatar placeholder */}
                            <div className="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-full bg-gradient-to-r from-blue-400 to-indigo-500 font-medium text-white">
                                {teacher.name?.charAt(0).toUpperCase() || "?"}
                            </div>

                            {/* Teacher info */}
                            <div>
                                <h3 className="font-medium text-gray-900 group-hover:text-blue-700">
                                    {teacher.name}
                                </h3>
                                <div className="mt-1 flex flex-wrap gap-2">
                                    {teacher.specialization && (
                                        <span className="inline-flex items-center rounded bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-800">
                      {teacher.specialization}
                    </span>
                                    )}
                                    {teacher.role && (
                                        <span className="inline-flex items-center rounded bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-800">
                      {teacher.role}
                    </span>
                                    )}
                                </div>
                            </div>
                        </div>

                        {onTeacherClick && (
                            <svg
                                xmlns="http://www.w3.org/2000/svg"
                                className="h-5 w-5 text-gray-400 transition-all duration-200 group-hover:translate-x-1 group-hover:transform group-hover:text-blue-500"
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
                        )}
                    </div>
                </div>
            ))}
        </div>
    );
}