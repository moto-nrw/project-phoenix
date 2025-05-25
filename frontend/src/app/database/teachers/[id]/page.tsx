"use client";

import { useSession, getSession } from "next-auth/react";
import { redirect, useRouter, useParams } from "next/navigation";
import { useState, useEffect, useCallback } from "react";
import { PageHeader, SectionTitle } from "@/components/dashboard";
import { teacherService, type Teacher } from "@/lib/teacher-api";
import { authService } from "@/lib/auth-service";
import type { Activity } from "@/lib/activity-helpers";
import type { Role, Permission, Account } from "@/lib/auth-helpers";
import { DeleteModal, Button } from "@/components/ui";
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

    // Account and auth state
    const [account, setAccount] = useState<Account | null>(null);
    const [accountRoles, setAccountRoles] = useState<Role[]>([]);
    const [accountPermissions, setAccountPermissions] = useState<Permission[]>([]);
    const [rolePermissions, setRolePermissions] = useState<Permission[]>([]);
    const [effectivePermissions, setEffectivePermissions] = useState<Permission[]>([]);
    const [allRoles, setAllRoles] = useState<Role[]>([]);
    const [allPermissions, setAllPermissions] = useState<Permission[]>([]);
    const [loadingAccount, setLoadingAccount] = useState(false);
    const [loadingRoles, setLoadingRoles] = useState(false);
    const [loadingPermissions, setLoadingPermissions] = useState(false);
    const [loadingAllPermissions, setLoadingAllPermissions] = useState(false);
    const [showPermissionManagement, setShowPermissionManagement] = useState(false);
    
    // User authorization state - default to false for security
    const [hasAuthManagePermission, setHasAuthManagePermission] = useState(false);
    const [loadingUserPermissions, setLoadingUserPermissions] = useState(true);

    // Account state - removed account creation variables since teachers always have accounts

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
                
                setTeacher(data);
                setError(null);
            } catch {
                setError(
                    "Fehler beim Laden der Lehrerdaten. Bitte versuchen Sie es später erneut.",
                );
                setTeacher(null);
            }
        } catch {
            setError(
                "Fehler beim Laden der Lehrerdaten. Bitte versuchen Sie es später erneut.",
            );
            setTeacher(null);
        } finally {
            setLoading(false);
        }
    }, [id]);

    // Function to fetch account roles - with improved error handling and response parsing
    const fetchAccountRoles = useCallback(async (accountId: string) => {
        // Skip if accountId is undefined or invalid
        if (!accountId || accountId === "undefined") {
            setLoadingRoles(false);
            return;
        }
        
        try {
            setLoadingRoles(true);
            
            // Get session for auth token
            const session = await getSession();
            if (!session?.user?.token) {
                setLoadingRoles(false);
                return;
            }
            
            // Call the roles endpoint directly
            const rolesUrl = `/api/auth/accounts/${accountId}/roles`;
            
            try {
                const rolesResponse = await fetch(rolesUrl, {
                    credentials: "include",
                    headers: {
                        Authorization: `Bearer ${session.user.token}`,
                        "Content-Type": "application/json",
                    },
                });
                
                if (rolesResponse.ok) {
                    const rolesData = await rolesResponse.json() as { data?: { data?: Role[] } | Role[] } | Role[];
                    
                    // Extract roles from the response, handling different formats
                    let roles: Role[] = [];
                    
                    if (typeof rolesData === 'object' && 'data' in rolesData && rolesData.data) {
                        if (typeof rolesData.data === 'object' && 'data' in rolesData.data && Array.isArray(rolesData.data.data)) {
                            // Double-nested: { data: { data: [] } }
                            roles = rolesData.data.data;
                        } else if (Array.isArray(rolesData.data)) {
                            // Single-nested: { data: [] }
                            roles = rolesData.data;
                        }
                    } else if (Array.isArray(rolesData)) {
                        // Direct array
                        roles = rolesData;
                    }
                    
                    setAccountRoles(roles);
                    
                    // Calculate effective permissions from roles
                    const permissionsFromRoles: Permission[] = [];
                    
                    for (const role of roles) {
                        try {
                            const rolePermissionsUrl = `/api/auth/roles/${role.id}/permissions`;
                            
                            const rolePermissionsResponse = await fetch(rolePermissionsUrl, {
                                credentials: "include",
                                headers: {
                                    Authorization: `Bearer ${session.user.token}`,
                                    "Content-Type": "application/json",
                                },
                            });
                            
                            if (rolePermissionsResponse.ok) {
                                const rolePermissionsData = await rolePermissionsResponse.json() as { data?: { data?: Permission[] } | Permission[] } | Permission[];
                                
                                // Extract permissions from the response, handling different formats
                                let rolePermissions: Permission[] = [];
                                
                                if (typeof rolePermissionsData === 'object' && 'data' in rolePermissionsData && rolePermissionsData.data) {
                                    if (typeof rolePermissionsData.data === 'object' && 'data' in rolePermissionsData.data && Array.isArray(rolePermissionsData.data.data)) {
                                        // Double-nested: { data: { data: [] } }
                                        rolePermissions = rolePermissionsData.data.data;
                                    } else if (Array.isArray(rolePermissionsData.data)) {
                                        // Single-nested: { data: [] }
                                        rolePermissions = rolePermissionsData.data;
                                    }
                                } else if (Array.isArray(rolePermissionsData)) {
                                    // Direct array
                                    rolePermissions = rolePermissionsData;
                                }
                                
                                permissionsFromRoles.push(...rolePermissions);
                            }
                        } catch {
                            // Skip this role's permissions on error
                        }
                    }
                    
                    // Store role permissions separately for effective permissions calculation
                    setRolePermissions(permissionsFromRoles);
                } else {
                    setAccountRoles([]);
                    setRolePermissions([]); // Clear role permissions if roles fetch fails
                }
            } catch {
                setAccountRoles([]);
            }
        } catch {
            setAccountRoles([]);
            setRolePermissions([]); // Clear role permissions on error
        } finally {
            setLoadingRoles(false);
        }
    }, []);

    // Function to fetch account permissions - with improved error handling and response parsing
    const fetchAccountPermissions = useCallback(async (accountId: string) => {
        // Skip if accountId is undefined or invalid
        if (!accountId || accountId === "undefined") {
            setLoadingPermissions(false);
            return;
        }
        
        try {
            setLoadingPermissions(true);
            
            // Get session for auth token
            const session = await getSession();
            if (!session?.user?.token) {
                setLoadingPermissions(false);
                return;
            }
            
            // Call the direct permissions endpoint to get only direct permissions (not role-based)
            const permissionsUrl = `/api/auth/accounts/${accountId}/permissions/direct`;
            
            try {
                const permissionsResponse = await fetch(permissionsUrl, {
                    credentials: "include",
                    headers: {
                        Authorization: `Bearer ${session.user.token}`,
                        "Content-Type": "application/json",
                    },
                });
                
                if (permissionsResponse.ok) {
                    const permissionsData = await permissionsResponse.json() as { data?: { data?: Permission[] } | Permission[] } | Permission[];
                    
                    // Extract permissions from the response, handling different formats
                    let permissions: Permission[] = [];
                    
                    if (typeof permissionsData === 'object' && 'data' in permissionsData && permissionsData.data) {
                        if (typeof permissionsData.data === 'object' && 'data' in permissionsData.data && Array.isArray(permissionsData.data.data)) {
                            // Double nested structure: { data: { data: [] } }
                            permissions = permissionsData.data.data;
                        } else if (Array.isArray(permissionsData.data)) {
                            // Single nested structure: { data: [] }
                            permissions = permissionsData.data;
                        }
                    } else if (Array.isArray(permissionsData)) {
                        // Direct array response
                        permissions = permissionsData;
                    }
                    
                    // Filter out invalid permissions (missing ID or essential fields)
                    const validPermissions = permissions.filter(p => p?.id);
                    
                    setAccountPermissions(validPermissions);
                    
                    // Note: effective permissions are calculated in fetchAccountRoles
                    // since we need both permissions and roles to calculate them
                } else {
                    setAccountPermissions([]);
                }
            } catch {
                setAccountPermissions([]);
            }
        } catch {
            setAccountPermissions([]);
        } finally {
            setLoadingPermissions(false);
        }
    }, []);

    // Function to fetch account information - simplified and streamlined approach
    const fetchAccountInfo = useCallback(async () => {
        if (!teacher) {
            return;
        }
        
        try {
            setLoadingAccount(true);
            const session = await getSession();
            
            if (!session?.user?.token) {
                setLoadingAccount(false);
                return;
            }
            
            // SIMPLIFIED APPROACH: First check if we already have account_id from teacher data
            let accountId = null;
            
            // Check if teacher already has an account_id directly
            if (teacher.account_id) {
                accountId = teacher.account_id.toString();
            }
            
            // If no direct account_id, look it up via person_id
            if (!accountId && teacher.person_id) {
                try {
                    // Fetch the person data to get account_id
                    const personResponse = await fetch(`/api/users/${teacher.person_id}`, {
                        credentials: "include",
                        headers: {
                            Authorization: `Bearer ${session.user.token}`,
                            "Content-Type": "application/json",
                        },
                    });
                    
                    if (personResponse.ok) {
                        const personData = await personResponse.json() as { data?: { account_id?: number | string } } | { account_id?: number | string };
                        const person = typeof personData === 'object' && 'data' in personData && personData.data ? personData.data : personData;
                        
                        if (typeof person === 'object' && 'account_id' in person && person.account_id) {
                            accountId = person.account_id.toString();
                        }
                    }
                } catch {
                    // Just continue to the next method
                }
            }
            
            // If we found an account ID, fetch the account details
            if (accountId) {
                try {
                    const accountResponse = await fetch(`/api/auth/accounts/${accountId}`, {
                        credentials: "include",
                        headers: {
                            Authorization: `Bearer ${session.user.token}`,
                            "Content-Type": "application/json",
                        },
                    });
                    
                    if (accountResponse.ok) {
                        const accountData = await accountResponse.json() as { data?: Account } | Account;
                        const account = typeof accountData === 'object' && 'data' in accountData && accountData.data ? accountData.data : accountData as Account;
                        
                        if (typeof account === 'object' && 'id' in account && account.id) {
                            setAccount(account);
                            return;
                        }
                    }
                } catch {
                    // Just continue to the next method
                }
            }
            
            // Create a fallback account if we couldn't find one
            setAccount({
                id: accountId ?? "0",
                email: teacher.email ?? "user@example.com",
                username: teacher.first_name ?? "user",
                active: true,
                roles: [],
                permissions: [],
                createdAt: new Date().toISOString(),
                updatedAt: new Date().toISOString()
            });
            
        } catch {
            // In case of error, provide a fallback account
            setAccount({
                id: "0",
                email: "user@example.com",
                username: "user",
                active: true,
                roles: [],
                permissions: [],
                createdAt: new Date().toISOString(),
                updatedAt: new Date().toISOString()
            });
        } finally {
            setLoadingAccount(false);
        }
    }, [teacher]);

    // Function to fetch all available roles - with direct API call and detailed logging
    const fetchAllRoles = useCallback(async () => {
        try {
            // Get session for auth token
            const session = await getSession();
            if (!session?.user?.token) {
                return;
            }
            
            // Call the roles endpoint directly
            const allRolesUrl = `/api/auth/roles`;
            
            try {
                const allRolesResponse = await fetch(allRolesUrl, {
                    credentials: "include",
                    headers: {
                        Authorization: `Bearer ${session.user.token}`,
                        "Content-Type": "application/json",
                    },
                });
                
                if (allRolesResponse.ok) {
                    const allRolesData = await allRolesResponse.json() as { data?: Role[] } | Role[];
                    
                    // Extract roles from the response
                    let roles: Role[] = [];
                    
                    // Handle different response formats
                    if (typeof allRolesData === 'object' && 'data' in allRolesData && Array.isArray(allRolesData.data)) {
                        roles = allRolesData.data;
                    } else if (Array.isArray(allRolesData)) {
                        roles = allRolesData;
                    }
                    
                    setAllRoles(roles);
                } else {
                    setAllRoles([]);
                }
            } catch {
                setAllRoles([]);
            }
        } catch {
            setAllRoles([]);
        }
    }, []);

    // Function to fetch all available permissions for assignment
    const fetchAllPermissions = useCallback(async () => {
        try {
            setLoadingAllPermissions(true);
            
            const permissions = await authService.getAvailablePermissions();
            setAllPermissions(permissions);
        } catch {
            setAllPermissions([]);
        } finally {
            setLoadingAllPermissions(false);
        }
    }, []);

    // Function to handle teacher deletion
    const handleDeleteTeacher = async () => {
        if (!id) return;

        try {
            setIsDeleting(true);
            await teacherService.deleteTeacher(id as string);
            router.push("/database/teachers");
        } catch {
            setError(
                "Fehler beim Löschen des Lehrers. Bitte versuchen Sie es später erneut.",
            );
            setShowDeleteConfirm(false);
        } finally {
            setIsDeleting(false);
        }
    };

    // Removed handleCreateAccount function - no longer needed as teachers always have accounts

    // Function to assign role
    const handleAssignRole = async (roleId: string) => {
        if (!account) return;

        try {
            await authService.assignRoleToAccount(account.id, roleId);
            await fetchAccountRoles(account.id);
        } catch {
            setError("Fehler beim Zuweisen der Rolle.");
            
            // Still update the roles list to refresh state
            try {
                await fetchAccountRoles(account.id);
            } catch {
                // Ignore errors during refresh
            }
        }
    };

    // Function to remove role
    const handleRemoveRole = async (roleId: string) => {
        if (!account) return;

        try {
            await authService.removeRoleFromAccount(account.id, roleId);
            await fetchAccountRoles(account.id);
        } catch {
            setError("Fehler beim Entfernen der Rolle.");
            
            // Still update the roles list to refresh state
            try {
                await fetchAccountRoles(account.id);
            } catch {
                // Ignore errors during refresh
            }
        }
    };

    // Function to assign permission
    const handleAssignPermission = async (permissionId: string) => {
        if (!account) return;

        try {
            await authService.grantPermissionToAccount(account.id, permissionId);
            await fetchAccountPermissions(account.id);
        } catch {
            setError("Fehler beim Zuweisen der Berechtigung.");
            
            // Still update the permissions list to refresh state
            try {
                await fetchAccountPermissions(account.id);
            } catch {
                // Ignore errors during refresh
            }
        }
    };

    // Function to remove permission
    const handleRemovePermission = async (permissionId: string) => {
        if (!account) return;

        try {
            await authService.removePermissionFromAccount(account.id, permissionId);
            await fetchAccountPermissions(account.id);
        } catch {
            setError("Fehler beim Entfernen der Berechtigung.");
            
            // Still update the permissions list to refresh state
            try {
                await fetchAccountPermissions(account.id);
            } catch {
                // Ignore errors during refresh
            }
        }
    };

    // Get current user permissions
    const fetchUserPermissions = useCallback(async () => {
        try {
            setLoadingUserPermissions(true);
            
            // Get current user account
            const userAccount = await authService.getAccount();
            
            if (!userAccount?.id) {
                setHasAuthManagePermission(false);
                return;
            }
            
            // Try to fetch user permissions by attempting to access a protected endpoint
            // If it succeeds, user has permission; if it fails, they don't
            try {
                // Try to fetch all permissions - this requires permissions:read
                await authService.getAvailablePermissions();
                
                // If we got here, user has at least read permission
                // Now check for management permissions by trying to fetch roles
                try {
                    await authService.getRoles();
                    // Success means user has auth management permissions
                    setHasAuthManagePermission(true);
                } catch {
                    // User can read but not manage
                    setHasAuthManagePermission(false);
                }
            } catch {
                // User doesn't have permission to read auth data
                setHasAuthManagePermission(false);
            }
        } catch {
            // SECURITY: Always default to false - deny access unless explicitly allowed
            setHasAuthManagePermission(false);
        } finally {
            setLoadingUserPermissions(false);
        }
    }, []);

    // Handle tab change with data loading
    const handleTabChange = useCallback((tabId: string) => {
        // Security check: Don't allow switching to restricted tabs without permission
        if ((tabId === "roles" || tabId === "permissions") && !hasAuthManagePermission) {
            return;
        }
        
        setActiveTab(tabId);
        
        // Load data based on tab if account exists and user has permission
        if (account && hasAuthManagePermission) {
            if (tabId === "roles") {
                // Only fetch roles if we haven't already
                if (accountRoles.length === 0) {
                    void fetchAccountRoles(account.id);
                }
                
                // Only fetch all roles if we haven't already
                if (allRoles.length === 0) {
                    void fetchAllRoles();
                }
            } else if (tabId === "permissions") {
                // Fetch both direct permissions and role permissions
                // The effective permissions will be calculated automatically via useEffect
                if (accountPermissions.length === 0) {
                    void fetchAccountPermissions(account.id);
                }
                if (accountRoles.length === 0 || rolePermissions.length === 0) {
                    void fetchAccountRoles(account.id);
                }
                
                // Fetch all available permissions for assignment
                if (allPermissions.length === 0) {
                    void fetchAllPermissions();
                }
            }
        }
    }, [account, accountRoles, accountPermissions, rolePermissions, allRoles, allPermissions, hasAuthManagePermission, fetchAccountRoles, fetchAccountPermissions, fetchAllRoles, fetchAllPermissions]);

    // Initial data load
    useEffect(() => {
        void fetchTeacher();
        void fetchUserPermissions();
    }, [id, fetchTeacher, fetchUserPermissions]);

    // Fetch account info when teacher data is loaded, but only once
    useEffect(() => {
        // Only fetch account info if we have teacher data and don't already have account data
        if (teacher && !account && !loadingAccount) {
            void fetchAccountInfo();
        }
    }, [teacher, fetchAccountInfo, account, loadingAccount]);

    // Only fetch all roles and permissions if user has permission
    useEffect(() => {
        if (hasAuthManagePermission && !loadingUserPermissions) {
            void fetchAllRoles();
            void fetchAllPermissions();
        }
    }, [hasAuthManagePermission, loadingUserPermissions, fetchAllRoles, fetchAllPermissions]);

    // Calculate effective permissions when either direct permissions or role permissions change
    useEffect(() => {
        // Combine direct permissions and role permissions
        const allPermissions = [...accountPermissions, ...rolePermissions];
        
        // Filter out any invalid permissions and deduplicate by ID
        const validPermissions = allPermissions.filter(p => p?.id);
        const uniquePermissions = validPermissions.filter((permission, index, self) =>
            index === self.findIndex((p) => p.id === permission.id)
        );
        
        setEffectivePermissions(uniquePermissions);
    }, [accountPermissions, rolePermissions]);

    // Only show the main loading screen if we're loading the teacher
    // Not for other loading states which are handled within their tabs
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

    // Dynamic tabs based on permissions
    const tabs: Tab[] = [
        { id: "details", label: "Lehrerdetails" },
        { id: "account", label: "Benutzerkonto" },
        // Only show roles and permissions tabs if user has auth management permission
        ...(hasAuthManagePermission ? [
            { id: "roles", label: "Rollen" },
            { id: "permissions", label: "Berechtigungen" },
        ] : []),
    ];

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
                                onClick={() => handleTabChange(tab.id)}
                                disabled={loadingUserPermissions || (
                                    !hasAuthManagePermission && 
                                    (tab.id === "roles" || tab.id === "permissions")
                                )}
                                className={`whitespace-nowrap py-2 px-1 border-b-2 font-medium text-sm ${
                                    activeTab === tab.id
                                        ? "border-blue-500 text-blue-600"
                                        : loadingUserPermissions || (
                                            !hasAuthManagePermission && 
                                            (tab.id === "roles" || tab.id === "permissions")
                                          )
                                            ? "border-transparent text-gray-300 cursor-not-allowed"
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
                            
                            {account ? (
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
                                                } catch {
                                                    setError("Fehler beim Ändern des Kontostatus.");
                                                }
                                            }}
                                            disabled={account.id === "0"} // Disable for fallback accounts
                                        >
                                            {account.active ? "Deaktivieren" : "Aktivieren"}
                                        </Button>
                                    </div>
                                </div>
                            ) : (
                                <div>
                                    <p className="mb-4 text-gray-600">
                                        Keine Kontoinformationen gefunden. Bitte kontaktieren Sie den Administrator.
                                    </p>
                                </div>
                            )}
                        </div>
                    )}

                    {activeTab === "roles" && (
                        <div>
                            <h3 className="mb-4 text-lg font-semibold text-gray-800">
                                Rollen
                            </h3>
                            
                            {loadingUserPermissions ? (
                                <p>Überprüfe Berechtigungen...</p>
                            ) : !hasAuthManagePermission ? (
                                <div className="p-4 bg-yellow-50 rounded-lg text-yellow-800">
                                    <p className="font-medium">Fehlende Berechtigung</p>
                                    <p className="text-sm mt-1">Sie haben keine Berechtigung, Rollen zu verwalten.</p>
                                </div>
                            ) : !account ? (
                                <p className="text-gray-600">
                                    Keine Kontoinformationen gefunden.
                                </p>
                            ) : account.id === "0" ? (
                                <div className="p-4 bg-yellow-50 rounded-lg text-yellow-800">
                                    <p className="font-medium">Kein Benutzerkonto gefunden</p>
                                    <p className="text-sm mt-1">Für diesen Lehrer wurde kein Benutzerkonto gefunden.</p>
                                </div>
                            ) : loadingRoles ? (
                                <div className="flex items-center justify-center py-10">
                                    <div className="animate-pulse text-blue-500">Lade Rollen...</div>
                                </div>
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
                            <div className="flex items-center justify-between mb-4">
                                <h3 className="text-lg font-semibold text-gray-800">
                                    Berechtigungen
                                </h3>
                                {hasAuthManagePermission && account && account.id !== "0" && !loadingUserPermissions && (
                                    <button
                                        className={`p-1.5 rounded-lg transition-colors ${
                                            showPermissionManagement 
                                                ? "bg-gray-100 hover:bg-gray-200 text-gray-700" 
                                                : "bg-blue-100 hover:bg-blue-200 text-blue-700"
                                        }`}
                                        title={showPermissionManagement ? "Anzeigen" : "Verwalten"}
                                        onClick={() => {
                                            setShowPermissionManagement(!showPermissionManagement);
                                            // Load all permissions when enabling management mode
                                            if (!showPermissionManagement && allPermissions.length === 0) {
                                                void fetchAllPermissions();
                                            }
                                        }}
                                    >
                                        {showPermissionManagement ? "👁️" : "⚙️"}
                                    </button>
                                )}
                            </div>
                            
                            {loadingUserPermissions ? (
                                <p>Überprüfe Berechtigungen...</p>
                            ) : !hasAuthManagePermission ? (
                                <div className="p-4 bg-yellow-50 rounded-lg text-yellow-800">
                                    <p className="font-medium">Fehlende Berechtigung</p>
                                    <p className="text-sm mt-1">Sie haben keine Berechtigung, Berechtigungen einzusehen.</p>
                                </div>
                            ) : !account ? (
                                <p className="text-gray-600">
                                    Keine Kontoinformationen gefunden.
                                </p>
                            ) : account.id === "0" ? (
                                <div className="p-4 bg-yellow-50 rounded-lg text-yellow-800">
                                    <p className="font-medium">Kein Benutzerkonto gefunden</p>
                                    <p className="text-sm mt-1">Für diesen Lehrer wurde kein Benutzerkonto gefunden.</p>
                                </div>
                            ) : loadingPermissions ? (
                                <div className="flex items-center justify-center py-10">
                                    <div className="animate-pulse text-blue-500">Lade Berechtigungen...</div>
                                </div>
                            ) : (
                                <div>
                                    <div className="mb-6">
                                        <h4 className="mb-2 font-medium text-gray-700">Direkte Berechtigungen</h4>
                                        {accountPermissions.length === 0 ? (
                                            <p className="text-gray-500">Keine direkten Berechtigungen</p>
                                        ) : (
                                            <div className="space-y-2">
                                                {accountPermissions.map((permission) => (
                                                    <div key={permission.id} className={`rounded-lg border p-3 ${showPermissionManagement ? "flex items-center justify-between" : ""}`}>
                                                        <div>
                                                            <p className="font-medium">{permission.name}</p>
                                                            {permission.description && (
                                                                <p className="text-sm text-gray-600">{permission.description}</p>
                                                            )}
                                                            <p className="text-xs text-gray-500 mt-1">
                                                                {permission.resource}:{permission.action}
                                                            </p>
                                                        </div>
                                                        {showPermissionManagement && (
                                                            <Button
                                                                variant="outline_danger"
                                                                size="sm"
                                                                onClick={() => handleRemovePermission(permission.id)}
                                                            >
                                                                Entfernen
                                                            </Button>
                                                        )}
                                                    </div>
                                                ))}
                                            </div>
                                        )}
                                    </div>

                                    {showPermissionManagement && (
                                        <div className="mb-6 p-4 bg-blue-50 rounded-lg border-2 border-blue-200">
                                            <h4 className="mb-3 font-medium text-gray-700 flex items-center">
                                                <span className="mr-2">⚙️</span>
                                                Verfügbare Berechtigungen zuweisen
                                            </h4>
                                            {loadingAllPermissions ? (
                                                <div className="flex items-center justify-center py-10">
                                                    <div className="animate-pulse text-blue-500">Lade verfügbare Berechtigungen...</div>
                                                </div>
                                            ) : allPermissions.length === 0 ? (
                                                <p className="text-gray-500">Keine Berechtigungen verfügbar</p>
                                            ) : (
                                                <div className="space-y-2 max-h-64 overflow-y-auto">
                                                    {allPermissions
                                                        .filter((permission) => !accountPermissions.some((ap) => ap.id === permission.id))
                                                        .map((permission) => (
                                                            <div key={permission.id} className="flex items-center justify-between rounded-lg border bg-white p-3">
                                                                <div>
                                                                    <p className="font-medium text-sm">{permission.name}</p>
                                                                    {permission.description && (
                                                                        <p className="text-sm text-gray-600">{permission.description}</p>
                                                                    )}
                                                                    <p className="text-xs text-gray-500 mt-1">
                                                                        {permission.resource}:{permission.action}
                                                                    </p>
                                                                </div>
                                                                <Button
                                                                    variant="primary"
                                                                    size="sm"
                                                                    onClick={() => handleAssignPermission(permission.id)}
                                                                >
                                                                    Zuweisen
                                                                </Button>
                                                            </div>
                                                        ))}
                                                </div>
                                            )}
                                        </div>
                                    )}

                                    <div>
                                        <h4 className="mb-2 font-medium text-gray-700">Effektive Berechtigungen (inkl. Rollen)</h4>
                                        <p className="text-sm text-gray-600 mb-3">
                                            Alle Berechtigungen die dieser Benutzer hat, sowohl direkt zugewiesene als auch über Rollen.
                                        </p>
                                        {effectivePermissions.length === 0 ? (
                                            <p className="text-gray-500">Keine effektiven Berechtigungen</p>
                                        ) : (
                                            <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
                                                {effectivePermissions.map((permission) => {
                                                    // Check if this permission comes from direct assignment or role
                                                    const isDirect = accountPermissions.some(p => p.id === permission.id);
                                                    const isFromRole = rolePermissions.some(p => p.id === permission.id);
                                                    
                                                    return (
                                                        <div key={permission.id} className="rounded-lg border p-3">
                                                            <div className="flex items-start justify-between">
                                                                <div className="flex-1">
                                                                    <p className="font-medium text-sm">{permission.name}</p>
                                                                    <p className="text-xs text-gray-500">
                                                                        {permission.resource}:{permission.action}
                                                                    </p>
                                                                </div>
                                                                <div className="ml-2 flex flex-col gap-1">
                                                                    {isDirect && (
                                                                        <span className="inline-block rounded-full bg-blue-100 px-2 py-0.5 text-xs text-blue-800">
                                                                            Direkt
                                                                        </span>
                                                                    )}
                                                                    {isFromRole && (
                                                                        <span className="inline-block rounded-full bg-green-100 px-2 py-0.5 text-xs text-green-800">
                                                                            Rolle
                                                                        </span>
                                                                    )}
                                                                </div>
                                                            </div>
                                                        </div>
                                                    );
                                                })}
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

            {/* Create Account Modal removed - no longer needed as teachers always have accounts */}
        </div>
    );
}