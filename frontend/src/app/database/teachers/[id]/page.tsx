"use client";

import { useSession } from "next-auth/react";
import { redirect, useRouter, useParams } from "next/navigation";
import { useState, useEffect, useCallback } from "react";
import { PageHeader, SectionTitle } from "@/components/dashboard";
import { teacherService, type Teacher } from "@/lib/teacher-api";
import { authService } from "@/lib/auth-service";
import type { Activity } from "@/lib/activity-helpers";
import type { Role, Permission, Account } from "@/lib/auth-helpers";
import { DeleteModal, Button, Input } from "@/components/ui";
import { CreateAccountModal } from "@/components/teachers/create-account-modal";
import Link from "next/link";

// Tab interface
interface Tab {
    id: string;
    label: string;
}

export default function TeacherDetailsPage() {
    const router = useRouter();
    const params = useParams();
    const { id } = params;
    const [teacher, setTeacher] = useState<Teacher | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
    const [isDeleting, setIsDeleting] = useState(false);

    // Tab state
    const [activeTab, setActiveTab] = useState("details");
    const tabs: Tab[] = [
        { id: "details", label: "Lehrerdetails" },
        { id: "account", label: "Benutzerkonto" },
        { id: "roles", label: "Rollen" },
        { id: "permissions", label: "Berechtigungen" },
    ];

    // Account and auth state
    const [account, setAccount] = useState<Account | null>(null);
    const [accountRoles, setAccountRoles] = useState<Role[]>([]);
    const [accountPermissions, setAccountPermissions] = useState<Permission[]>([]);
    const [effectivePermissions, setEffectivePermissions] = useState<Permission[]>([]);
    const [allRoles, setAllRoles] = useState<Role[]>([]);
    const [loadingAccount, setLoadingAccount] = useState(false);
    const [loadingRoles, setLoadingRoles] = useState(false);
    const [loadingPermissions, setLoadingPermissions] = useState(false);

    // Account creation state
    const [showCreateAccount, setShowCreateAccount] = useState(false);
    const [creatingAccount, setCreatingAccount] = useState(false);
    const [createAccountError, setCreateAccountError] = useState<string | null>(null);

    const { status } = useSession({
        required: true,
        onUnauthenticated() {
            redirect("/");
        },
    });

    // Function to fetch the teacher details
    const fetchTeacher = useCallback(async () => {
        if (!id) return;

        try {
            setLoading(true);

            try {
                // Fetch teacher from API
                const data = await teacherService.getTeacher(id as string);
                console.log("Teacher data received:", data);
                setTeacher(data);
                setError(null);

                // Email is now handled in the create account modal
            } catch (apiErr) {
                console.error("API error when fetching teacher:", apiErr);
                setError(
                    "Fehler beim Laden der Lehrerdaten. Bitte versuchen Sie es später erneut.",
                );
                setTeacher(null);
            }
        } catch (err) {
            console.error("Error fetching teacher:", err);
            setError(
                "Fehler beim Laden der Lehrerdaten. Bitte versuchen Sie es später erneut.",
            );
            setTeacher(null);
        } finally {
            setLoading(false);
        }
    }, [id]);

    // Function to fetch account information
    const fetchAccountInfo = useCallback(async () => {
        if (!teacher || !teacher.email) return;

        try {
            setLoadingAccount(true);
            // Try to find account by email
            const accounts = await authService.getAccounts({ email: teacher.email });
            if (accounts && accounts.length > 0) {
                const firstAccount = accounts[0];
                if (firstAccount) {
                    setAccount(firstAccount);
                    await fetchAccountRoles(firstAccount.id);
                    await fetchAccountPermissions(firstAccount.id);
                }
            } else {
                setAccount(null);
                setAccountRoles([]);
                setAccountPermissions([]);
                setEffectivePermissions([]);
            }
        } catch (err) {
            console.error("Error fetching account info:", err);
        } finally {
            setLoadingAccount(false);
        }
    }, [teacher]);

    // Function to fetch account roles
    const fetchAccountRoles = async (accountId: string) => {
        try {
            setLoadingRoles(true);
            const roles = await authService.getAccountRoles(accountId);
            setAccountRoles(roles);

            // Calculate effective permissions from roles
            const permissionsFromRoles: Permission[] = [];
            for (const role of roles) {
                const rolePermissions = await authService.getRolePermissions(role.id);
                permissionsFromRoles.push(...rolePermissions);
            }
            
            // Combine with direct permissions (deduplicate)
            const allPermissions = [...permissionsFromRoles, ...accountPermissions];
            const uniquePermissions = allPermissions.filter((permission, index, self) =>
                index === self.findIndex((p) => p.id === permission.id)
            );
            setEffectivePermissions(uniquePermissions);
        } catch (err) {
            console.error("Error fetching account roles:", err);
        } finally {
            setLoadingRoles(false);
        }
    };

    // Function to fetch account permissions
    const fetchAccountPermissions = async (accountId: string) => {
        try {
            setLoadingPermissions(true);
            const permissions = await authService.getAccountPermissions(accountId);
            setAccountPermissions(permissions);
        } catch (err) {
            console.error("Error fetching account permissions:", err);
        } finally {
            setLoadingPermissions(false);
        }
    };

    // Function to fetch all available roles
    const fetchAllRoles = useCallback(async () => {
        try {
            const roles = await authService.getRoles();
            setAllRoles(roles);
        } catch (err) {
            console.error("Error fetching all roles:", err);
        }
    }, []);

    // Function to handle teacher deletion
    const handleDeleteTeacher = async () => {
        if (!id) return;

        try {
            setIsDeleting(true);
            await teacherService.deleteTeacher(id as string);
            router.push("/database/teachers");
        } catch (err) {
            console.error("Error deleting teacher:", err);
            setError(
                "Fehler beim Löschen des Lehrers. Bitte versuchen Sie es später erneut.",
            );
            setShowDeleteConfirm(false);
        } finally {
            setIsDeleting(false);
        }
    };

    // Function to create account
    const handleCreateAccount = async (email: string, password: string) => {
        if (!teacher) return;

        try {
            setCreatingAccount(true);
            const accountData = await authService.register({
                email: email,
                username: email.split('@')[0],
                name: teacher.first_name && teacher.last_name 
                    ? `${teacher.first_name} ${teacher.last_name}`
                    : teacher.name,
                password: password,
                confirmPassword: password,
            });

            setAccount(accountData);
            setShowCreateAccount(false);
            
            // Refresh teacher data to get updated email
            await fetchTeacher();
        } catch (err) {
            console.error("Error creating account:", err);
            setCreateAccountError(err instanceof Error ? err.message : "Fehler beim Erstellen des Benutzerkontos.");
        } finally {
            setCreatingAccount(false);
        }
    };

    // Function to assign role
    const handleAssignRole = async (roleId: string) => {
        if (!account) return;

        try {
            await authService.assignRoleToAccount(account.id, roleId);
            await fetchAccountRoles(account.id);
        } catch (err) {
            console.error("Error assigning role:", err);
            setError("Fehler beim Zuweisen der Rolle.");
        }
    };

    // Function to remove role
    const handleRemoveRole = async (roleId: string) => {
        if (!account) return;

        try {
            await authService.removeRoleFromAccount(account.id, roleId);
            await fetchAccountRoles(account.id);
        } catch (err) {
            console.error("Error removing role:", err);
            setError("Fehler beim Entfernen der Rolle.");
        }
    };

    // Initial data load
    useEffect(() => {
        void fetchTeacher();
    }, [id, fetchTeacher]);

    // Fetch account info when teacher data is loaded
    useEffect(() => {
        if (teacher) {
            void fetchAccountInfo();
        }
    }, [teacher, fetchAccountInfo]);

    // Fetch all roles on component mount
    useEffect(() => {
        void fetchAllRoles();
    }, [fetchAllRoles]);

    if (status === "loading" || loading) {
        return (
            <div className="flex min-h-screen items-center justify-center">
                <p>Loading...</p>
            </div>
        );
    }

    // Show error if loading failed
    if (error) {
        return (
            <div className="flex min-h-screen flex-col items-center justify-center p-4">
                <div className="max-w-md rounded-lg bg-red-50 p-4 text-red-800">
                    <h2 className="mb-2 font-semibold">Fehler</h2>
                    <p>{error}</p>
                    <button
                        onClick={() => fetchTeacher()}
                        className="mt-4 rounded bg-red-100 px-4 py-2 text-red-800 transition-colors hover:bg-red-200"
                    >
                        Erneut versuchen
                    </button>
                </div>
            </div>
        );
    }

    if (!teacher) {
        return (
            <div className="flex min-h-screen flex-col items-center justify-center p-4">
                <div className="max-w-md rounded-lg bg-orange-50 p-4 text-orange-800">
                    <h2 className="mb-2 font-semibold">Lehrer nicht gefunden</h2>
                    <p>Der angeforderte Lehrer konnte nicht gefunden werden.</p>
                    <Link href="/database/teachers">
                        <button className="mt-4 rounded bg-orange-100 px-4 py-2 text-orange-800 transition-colors hover:bg-orange-200">
                            Zurück zur Übersicht
                        </button>
                    </Link>
                </div>
            </div>
        );
    }

    return (
        <div className="min-h-screen">
            <PageHeader title={teacher.name} backUrl="/database/teachers" />

            <main className="mx-auto max-w-4xl p-4">
                <div className="mb-8">
                    <SectionTitle title="Lehrerdetails" />
                </div>

                {/* Tabs */}
                <div className="mb-6 border-b border-gray-200">
                    <nav className="-mb-px flex space-x-8">
                        {tabs.map((tab) => (
                            <button
                                key={tab.id}
                                onClick={() => setActiveTab(tab.id)}
                                className={`whitespace-nowrap py-2 px-1 border-b-2 font-medium text-sm ${
                                    activeTab === tab.id
                                        ? "border-blue-500 text-blue-600"
                                        : "border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300"
                                }`}
                            >
                                {tab.label}
                            </button>
                        ))}
                    </nav>
                </div>

                {/* Tab Content */}
                <div className="mb-6 rounded-lg border border-gray-100 bg-white p-6 shadow-sm">
                    {activeTab === "details" && (
                        <>
                            {/* General Information */}
                            <div className="mb-8 grid grid-cols-1 gap-6 md:grid-cols-2">
                                <div>
                                    <h3 className="mb-4 text-lg font-semibold text-gray-800">
                                        Persönliche Informationen
                                    </h3>
                                    <dl className="space-y-2">
                                        <div className="flex flex-col">
                                            <dt className="text-sm text-gray-500">Name</dt>
                                            <dd className="font-medium">{teacher.name}</dd>
                                        </div>

                                        <div className="flex flex-col">
                                            <dt className="text-sm text-gray-500">Vorname</dt>
                                            <dd className="font-medium">{teacher.first_name}</dd>
                                        </div>

                                        <div className="flex flex-col">
                                            <dt className="text-sm text-gray-500">Nachname</dt>
                                            <dd className="font-medium">{teacher.last_name}</dd>
                                        </div>

                                        {teacher.tag_id && (
                                            <div className="flex flex-col">
                                                <dt className="text-sm text-gray-500">Tag ID</dt>
                                                <dd className="font-medium">{teacher.tag_id}</dd>
                                            </div>
                                        )}
                                    </dl>
                                </div>

                                <div>
                                    <h3 className="mb-4 text-lg font-semibold text-gray-800">
                                        Berufliche Details
                                    </h3>
                                    <dl className="space-y-2">
                                        <div className="flex flex-col">
                                            <dt className="text-sm text-gray-500">Fachgebiet</dt>
                                            <dd className="font-medium">{teacher.specialization}</dd>
                                        </div>

                                        {teacher.role && (
                                            <div className="flex flex-col">
                                                <dt className="text-sm text-gray-500">Rolle</dt>
                                                <dd className="font-medium">{teacher.role}</dd>
                                            </div>
                                        )}

                                        {teacher.qualifications && (
                                            <div className="flex flex-col">
                                                <dt className="text-sm text-gray-500">Qualifikationen</dt>
                                                <dd className="font-medium">{teacher.qualifications}</dd>
                                            </div>
                                        )}

                                        {teacher.staff_notes && (
                                            <div className="flex flex-col">
                                                <dt className="text-sm text-gray-500">Notizen</dt>
                                                <dd className="font-medium">{teacher.staff_notes}</dd>
                                            </div>
                                        )}
                                    </dl>
                                </div>
                            </div>

                            {/* Date Information */}
                            <div className="mb-6 grid grid-cols-1 gap-4 border-t border-gray-100 pt-6 sm:grid-cols-2">
                                <div className="flex flex-col">
                                    <span className="text-sm text-gray-500">Erstellt am</span>
                                    <span className="font-medium">
                                        {teacher.created_at
                                            ? new Date(teacher.created_at).toLocaleDateString("de-DE")
                                            : "Unbekannt"}
                                    </span>
                                </div>

                                <div className="flex flex-col">
                                    <span className="text-sm text-gray-500">Aktualisiert am</span>
                                    <span className="font-medium">
                                        {teacher.updated_at
                                            ? new Date(teacher.updated_at).toLocaleDateString("de-DE")
                                            : "Unbekannt"}
                                    </span>
                                </div>
                            </div>

                            {/* Action Buttons */}
                            <div className="mt-6 flex flex-col gap-3 sm:flex-row">
                                <Link href={`/database/teachers/${teacher.id}/edit`}>
                                    <button className="w-full rounded-lg bg-blue-500 px-4 py-2 text-white transition-colors hover:bg-blue-600 sm:w-auto">
                                        Lehrer bearbeiten
                                    </button>
                                </Link>

                                {teacher.activities && teacher.activities.length > 0 && (
                                    <Link href={`/database/teachers/${teacher.id}/activities`}>
                                        <button className="w-full rounded-lg bg-green-500 px-4 py-2 text-white transition-colors hover:bg-green-600 sm:w-auto">
                                            Aktivitäten anzeigen ({teacher.activities.length})
                                        </button>
                                    </Link>
                                )}

                                <button
                                    onClick={() => setShowDeleteConfirm(true)}
                                    className="w-full rounded-lg bg-red-500 px-4 py-2 text-white transition-colors hover:bg-red-600 sm:w-auto"
                                >
                                    Lehrer löschen
                                </button>
                            </div>
                        </>
                    )}

                    {activeTab === "account" && (
                        <div>
                            <h3 className="mb-4 text-lg font-semibold text-gray-800">
                                Benutzerkonto
                            </h3>
                            
                            {loadingAccount ? (
                                <p>Lade Kontoinformationen...</p>
                            ) : account ? (
                                <div>
                                    <dl className="space-y-2">
                                        <div className="flex flex-col">
                                            <dt className="text-sm text-gray-500">E-Mail</dt>
                                            <dd className="font-medium">{account.email}</dd>
                                        </div>
                                        <div className="flex flex-col">
                                            <dt className="text-sm text-gray-500">Benutzername</dt>
                                            <dd className="font-medium">{account.username}</dd>
                                        </div>
                                        <div className="flex flex-col">
                                            <dt className="text-sm text-gray-500">Status</dt>
                                            <dd className="font-medium">
                                                {account.active ? (
                                                    <span className="text-green-600">Aktiv</span>
                                                ) : (
                                                    <span className="text-red-600">Inaktiv</span>
                                                )}
                                            </dd>
                                        </div>
                                    </dl>

                                    <div className="mt-6 flex gap-3">
                                        <Button
                                            variant={account.active ? "danger" : "primary"}
                                            onClick={async () => {
                                                try {
                                                    if (account.active) {
                                                        await authService.deactivateAccount(account.id);
                                                    } else {
                                                        await authService.activateAccount(account.id);
                                                    }
                                                    await fetchAccountInfo();
                                                } catch (err) {
                                                    console.error("Error toggling account status:", err);
                                                    setError("Fehler beim Ändern des Kontostatus.");
                                                }
                                            }}
                                        >
                                            {account.active ? "Deaktivieren" : "Aktivieren"}
                                        </Button>
                                    </div>
                                </div>
                            ) : (
                                <div>
                                    <p className="mb-4 text-gray-600">
                                        Dieser Lehrer hat noch kein Benutzerkonto.
                                    </p>
                                    <Button
                                        variant="primary"
                                        onClick={() => setShowCreateAccount(true)}
                                    >
                                        Konto erstellen
                                    </Button>
                                </div>
                            )}
                        </div>
                    )}

                    {activeTab === "roles" && (
                        <div>
                            <h3 className="mb-4 text-lg font-semibold text-gray-800">
                                Rollen
                            </h3>
                            
                            {!account ? (
                                <p className="text-gray-600">
                                    Erstellen Sie zuerst ein Benutzerkonto, um Rollen zuzuweisen.
                                </p>
                            ) : loadingRoles ? (
                                <p>Lade Rollen...</p>
                            ) : (
                                <div>
                                    <div className="mb-6">
                                        <h4 className="mb-2 font-medium text-gray-700">Zugewiesene Rollen</h4>
                                        {accountRoles.length === 0 ? (
                                            <p className="text-gray-500">Keine Rollen zugewiesen</p>
                                        ) : (
                                            <div className="space-y-2">
                                                {accountRoles.map((role) => (
                                                    <div key={role.id} className="flex items-center justify-between rounded-lg border p-3">
                                                        <div>
                                                            <p className="font-medium">{role.name}</p>
                                                            {role.description && (
                                                                <p className="text-sm text-gray-600">{role.description}</p>
                                                            )}
                                                        </div>
                                                        <Button
                                                            variant="outline_danger"
                                                            size="sm"
                                                            onClick={() => handleRemoveRole(role.id)}
                                                        >
                                                            Entfernen
                                                        </Button>
                                                    </div>
                                                ))}
                                            </div>
                                        )}
                                    </div>

                                    <div>
                                        <h4 className="mb-2 font-medium text-gray-700">Verfügbare Rollen</h4>
                                        {allRoles.length === 0 ? (
                                            <p className="text-gray-500">Keine Rollen verfügbar</p>
                                        ) : (
                                            <div className="space-y-2">
                                                {allRoles
                                                    .filter((role) => !accountRoles.some((ar) => ar.id === role.id))
                                                    .map((role) => (
                                                        <div key={role.id} className="flex items-center justify-between rounded-lg border p-3">
                                                            <div>
                                                                <p className="font-medium">{role.name}</p>
                                                                {role.description && (
                                                                    <p className="text-sm text-gray-600">{role.description}</p>
                                                                )}
                                                            </div>
                                                            <Button
                                                                variant="primary"
                                                                size="sm"
                                                                onClick={() => handleAssignRole(role.id)}
                                                            >
                                                                Zuweisen
                                                            </Button>
                                                        </div>
                                                    ))}
                                            </div>
                                        )}
                                    </div>
                                </div>
                            )}
                        </div>
                    )}

                    {activeTab === "permissions" && (
                        <div>
                            <h3 className="mb-4 text-lg font-semibold text-gray-800">
                                Berechtigungen
                            </h3>
                            
                            {!account ? (
                                <p className="text-gray-600">
                                    Erstellen Sie zuerst ein Benutzerkonto, um Berechtigungen anzuzeigen.
                                </p>
                            ) : loadingPermissions ? (
                                <p>Lade Berechtigungen...</p>
                            ) : (
                                <div>
                                    <div className="mb-6">
                                        <h4 className="mb-2 font-medium text-gray-700">Direkte Berechtigungen</h4>
                                        {accountPermissions.length === 0 ? (
                                            <p className="text-gray-500">Keine direkten Berechtigungen</p>
                                        ) : (
                                            <div className="space-y-2">
                                                {accountPermissions.map((permission) => (
                                                    <div key={permission.id} className="rounded-lg border p-3">
                                                        <p className="font-medium">{permission.name}</p>
                                                        {permission.description && (
                                                            <p className="text-sm text-gray-600">{permission.description}</p>
                                                        )}
                                                        <p className="text-xs text-gray-500 mt-1">
                                                            {permission.resource}:{permission.action}
                                                        </p>
                                                    </div>
                                                ))}
                                            </div>
                                        )}
                                    </div>

                                    <div>
                                        <h4 className="mb-2 font-medium text-gray-700">Effektive Berechtigungen (inkl. Rollen)</h4>
                                        {effectivePermissions.length === 0 ? (
                                            <p className="text-gray-500">Keine effektiven Berechtigungen</p>
                                        ) : (
                                            <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
                                                {effectivePermissions.map((permission) => (
                                                    <div key={permission.id} className="rounded-lg border p-3">
                                                        <p className="font-medium text-sm">{permission.name}</p>
                                                        <p className="text-xs text-gray-500">
                                                            {permission.resource}:{permission.action}
                                                        </p>
                                                    </div>
                                                ))}
                                            </div>
                                        )}
                                    </div>
                                </div>
                            )}
                        </div>
                    )}

                    {/* Delete Confirmation Dialog */}
                    <DeleteModal
                        isOpen={showDeleteConfirm}
                        onClose={() => setShowDeleteConfirm(false)}
                        onDelete={handleDeleteTeacher}
                        title="Lehrer löschen"
                        isDeleting={isDeleting}
                    >
                        <p>
                            Sind Sie sicher, dass Sie den Lehrer &quot;{teacher.name}
                            &quot; löschen möchten? Dies kann nicht rückgängig gemacht werden.
                        </p>
                        {teacher.activities && teacher.activities.length > 0 && (
                            <p className="mt-2 text-sm text-red-600">
                                Achtung: Dieser Lehrer ist für {teacher.activities.length} Aktivitäten verantwortlich. Das Löschen kann Auswirkungen auf diese Aktivitäten haben.
                            </p>
                        )}
                    </DeleteModal>
                </div>

                {/* Activities Section (if applicable) */}
                {activeTab === "details" && teacher.activities && teacher.activities.length > 0 && (
                    <div className="mb-6 rounded-lg border border-gray-100 bg-white p-6 shadow-sm">
                        <h3 className="mb-4 text-lg font-semibold text-gray-800">
                            Geleitete Aktivitäten
                        </h3>
                        <div className="space-y-2">
                            {teacher.activities.map((activity: Activity) => (
                                <div
                                    key={activity.id}
                                    className="group rounded-lg border border-gray-100 p-3 transition-all hover:border-blue-200 hover:bg-blue-50"
                                >
                                    <Link href={`/database/activities/${activity.id}`}>
                                        <div className="flex items-center justify-between">
                                            <div>
                                                <span className="font-medium text-gray-900 transition-colors group-hover:text-blue-600">
                                                    {activity.name}
                                                </span>
                                                {activity.category_name && (
                                                    <span className="ml-2 text-sm text-gray-500">
                                                        Kategorie: {activity.category_name}
                                                    </span>
                                                )}
                                            </div>
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
                                        </div>
                                    </Link>
                                </div>
                            ))}
                        </div>
                    </div>
                )}
            </main>

            {/* Create Account Modal */}
            <CreateAccountModal
                isOpen={showCreateAccount}
                onClose={() => {
                    setShowCreateAccount(false);
                    setCreateAccountError(null);
                }}
                onSubmit={handleCreateAccount}
                isLoading={creatingAccount}
                error={createAccountError}
            />
        </div>
    );
}