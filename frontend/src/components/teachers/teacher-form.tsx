import { useState, useEffect, useRef } from "react";
import type { Teacher } from "@/lib/teacher-api";
import { authService } from "@/lib/auth-service";

interface RoleOption {
  id: number;
  name: string;
}

interface TeacherFormProps {
  readonly initialData: Partial<Teacher>;
  readonly onSubmitAction: (
    data: Partial<Teacher> & { password?: string; role_id?: number },
  ) => Promise<void>;
  readonly onCancelAction: () => void;
  readonly isLoading: boolean;
  readonly formTitle?: string;
  readonly submitLabel?: string;
  readonly rfidCards?: ReadonlyArray<{ id: string; label: string }>;
  // When false, render without outer card container/headline (for edit modal minimal UI)
  readonly wrapInCard?: boolean;
  // Show/hide RFID UI block (kept off by default to avoid confusion)
  readonly showRFID?: boolean;
}

export function TeacherForm({
  initialData,
  onSubmitAction,
  onCancelAction,
  isLoading,
  formTitle = "Details der pädagogischen Fachkraft",
  submitLabel = "Speichern",
  rfidCards: _rfidCards = [],
  wrapInCard = true,
  showRFID = false,
}: TeacherFormProps) {
  // Form state
  const [firstName, setFirstName] = useState(initialData.first_name ?? "");
  const [lastName, setLastName] = useState(initialData.last_name ?? "");
  const [email, setEmail] = useState(initialData.email ?? "");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [role, setRole] = useState(initialData.role ?? "");
  const [tagId, setTagId] = useState(initialData.tag_id ?? "");

  // Role selection state (system roles, not job titles)
  const [roleId, setRoleId] = useState<number | undefined>(undefined);
  const [roles, setRoles] = useState<RoleOption[]>([]);
  const [isLoadingRoles, setIsLoadingRoles] = useState(false);

  // Form validation
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [submitError, setSubmitError] = useState<string | null>(null);

  // Store a reference to track when we need to reset the form
  const prevIdRef = useRef(initialData?.id);

  // Fetch roles on mount (only for new teachers)
  useEffect(() => {
    if (initialData.id) return; // Skip for editing existing teachers

    let cancelled = false;
    async function fetchRoles() {
      try {
        setIsLoadingRoles(true);
        const roleList = await authService.getRoles();
        if (cancelled) return;

        const options = roleList
          .map<RoleOption>((role) => ({
            id: Number(role.id),
            name: role.name ?? `Rolle ${role.id}`,
          }))
          .filter((role) => !Number.isNaN(role.id));

        setRoles(options);
      } catch (err) {
        console.error("Failed to load roles", err);
      } finally {
        if (!cancelled) {
          setIsLoadingRoles(false);
        }
      }
    }

    void fetchRoles();
    return () => {
      cancelled = true;
    };
  }, [initialData.id]);

  // Reset form when the teacher being edited changes
  useEffect(() => {
    const currentId = initialData?.id;

    // Only reset if we're switching to a different teacher or creating new
    if (prevIdRef.current !== currentId) {
      // Reset all form fields
      setFirstName(initialData.first_name ?? "");
      setLastName(initialData.last_name ?? "");
      setEmail(initialData.email ?? "");
      setRole(initialData.role ?? "");
      setTagId(initialData.tag_id ?? "");
      // Reset password fields
      setPassword("");
      setConfirmPassword("");
      // Reset errors
      setErrors({});
      setSubmitError(null);

      prevIdRef.current = currentId;
    }
  }, [initialData]);

  // Validate form
  const validateForm = () => {
    const newErrors: Record<string, string> = {};

    if (!firstName.trim()) {
      newErrors.firstName = "Vorname ist erforderlich";
    }

    if (!lastName.trim()) {
      newErrors.lastName = "Nachname ist erforderlich";
    }

    // Email and role validation only for new teachers
    if (!initialData.id) {
      if (!email.trim()) {
        newErrors.email = "E-Mail ist erforderlich";
      } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
        newErrors.email = "Ungültige E-Mail-Adresse";
      }

      if (!roleId || roleId <= 0) {
        newErrors.roleId = "Bitte wähle eine Rolle aus";
      }

      if (!password) {
        newErrors.password = "Passwort ist erforderlich";
      } else if (password.length < 8) {
        newErrors.password = "Passwort muss mindestens 8 Zeichen lang sein";
      } else if (!/[A-Z]/.test(password)) {
        newErrors.password =
          "Passwort muss mindestens einen Großbuchstaben enthalten";
      } else if (!/[a-z]/.test(password)) {
        newErrors.password =
          "Passwort muss mindestens einen Kleinbuchstaben enthalten";
      } else if (!/[0-9]/.test(password)) {
        newErrors.password = "Passwort muss mindestens eine Zahl enthalten";
      } else if (!/[^a-zA-Z0-9]/.test(password)) {
        newErrors.password =
          "Passwort muss mindestens ein Sonderzeichen enthalten";
      }

      if (!confirmPassword) {
        newErrors.confirmPassword = "Passwortbestätigung ist erforderlich";
      } else if (password !== confirmPassword) {
        newErrors.confirmPassword = "Passwörter stimmen nicht überein";
      }
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  // Handle form submission
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitError(null);

    // Validate form
    if (!validateForm()) {
      return;
    }

    try {
      // Prepare data for submission
      const formData: Partial<Teacher> & {
        password?: string;
        role_id?: number;
      } = {
        first_name: firstName.trim(),
        last_name: lastName.trim(),
        email: email.trim() || undefined,
        role: role.trim() || null,
        tag_id: tagId || null, // Use the TagID directly
        // Preserve existing IDs when editing
        ...(initialData.id && { id: initialData.id }),
        ...(initialData.person_id && { person_id: initialData.person_id }),
        // Include is_teacher flag
        is_teacher: true,
      };

      // Submit the form data

      // Include password and role_id for new teachers
      if (!initialData.id) {
        formData.password = password;
        formData.role_id = roleId;
      }

      // Submit the form
      await onSubmitAction(formData);
    } catch (err) {
      console.error("Error submitting form:", err);
      setSubmitError(
        "Es ist ein Fehler aufgetreten. Bitte versuchen Sie es später erneut.",
      );
    }
  };

  return (
    <div
      className={
        wrapInCard
          ? "rounded-lg border border-gray-100 bg-white p-4 shadow-sm md:p-6"
          : ""
      }
    >
      {wrapInCard && formTitle && (
        <h3 className="mb-4 text-base font-semibold text-gray-800 md:mb-6 md:text-lg">
          {formTitle}
        </h3>
      )}

      {submitError && (
        <div className="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-xs text-red-800 md:mb-6 md:p-4 md:text-sm">
          {submitError}
        </div>
      )}

      <form
        onSubmit={handleSubmit}
        noValidate
        className="space-y-4 md:space-y-6"
      >
        {/* Personal Information Section */}
        <div className="rounded-xl border border-gray-100 bg-orange-50/30 p-3 md:p-4">
          <h4 className="mb-3 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-4 md:text-sm">
            <svg
              className="h-3.5 w-3.5 text-orange-600 md:h-4 md:w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
              />
            </svg>
            Persönliche Informationen
          </h4>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2 md:gap-4">
            {/* First Name */}
            <div>
              <label
                htmlFor="firstName"
                className="mb-1 block text-xs font-medium text-gray-700"
              >
                Vorname <span className="text-red-500">*</span>
              </label>
              <input
                type="text"
                id="firstName"
                name="firstName"
                value={firstName}
                onChange={(e) => setFirstName(e.target.value)}
                className={`w-full rounded-lg border ${
                  errors.firstName
                    ? "border-red-300 bg-red-50"
                    : "border-gray-200 bg-white focus:border-[#F78C10] focus:ring-1 focus:ring-[#F78C10]"
                } px-3 py-2 text-sm transition-colors`}
                disabled={isLoading}
                autoComplete="given-name"
              />
              {errors.firstName && (
                <p className="mt-1 text-xs text-red-600">{errors.firstName}</p>
              )}
            </div>

            {/* Last Name */}
            <div>
              <label
                htmlFor="lastName"
                className="mb-1 block text-xs font-medium text-gray-700"
              >
                Nachname <span className="text-red-500">*</span>
              </label>
              <input
                type="text"
                id="lastName"
                name="lastName"
                value={lastName}
                onChange={(e) => setLastName(e.target.value)}
                className={`w-full rounded-lg border ${
                  errors.lastName
                    ? "border-red-300 bg-red-50"
                    : "border-gray-200 bg-white focus:border-[#F78C10] focus:ring-1 focus:ring-[#F78C10]"
                } px-3 py-2 text-sm transition-colors`}
                disabled={isLoading}
                autoComplete="family-name"
              />
              {errors.lastName && (
                <p className="mt-1 text-xs text-red-600">{errors.lastName}</p>
              )}
            </div>

            {/* Email - only for new teachers */}
            {!initialData.id && (
              <div>
                <label
                  htmlFor="email"
                  className="mb-1 block text-xs font-medium text-gray-700"
                >
                  E-Mail <span className="text-red-500">*</span>
                </label>
                <input
                  type="email"
                  id="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className={`w-full rounded-lg border ${
                    errors.email
                      ? "border-red-300 bg-red-50"
                      : "border-gray-200 bg-white focus:border-[#F78C10] focus:ring-1 focus:ring-[#F78C10]"
                  } px-3 py-2 text-sm transition-colors`}
                  disabled={isLoading}
                />
                {errors.email && (
                  <p className="mt-1 text-xs text-red-600">{errors.email}</p>
                )}
              </div>
            )}

            {/* Role Selection - only for new teachers */}
            {!initialData.id && (
              <div>
                <label
                  htmlFor="role-select"
                  className="mb-1 block text-xs font-medium text-gray-700"
                >
                  System-Rolle <span className="text-red-500">*</span>
                </label>
                <div className="relative">
                  <select
                    id="role-select"
                    value={roleId ?? ""}
                    onChange={(e) => {
                      const value = Number(e.target.value);
                      setRoleId(e.target.value === "" ? undefined : value);
                    }}
                    className={`w-full appearance-none rounded-lg border ${
                      errors.roleId
                        ? "border-red-300 bg-red-50"
                        : "border-gray-200 bg-white focus:border-[#F78C10] focus:ring-1 focus:ring-[#F78C10]"
                    } px-3 py-2 pr-10 text-sm transition-colors`}
                    disabled={isLoading || isLoadingRoles}
                  >
                    <option value="" disabled>
                      {isLoadingRoles ? "Lade Rollen..." : "Rolle auswählen..."}
                    </option>
                    {roles.map((role) => (
                      <option key={role.id} value={role.id}>
                        {role.name}
                      </option>
                    ))}
                  </select>
                  <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3">
                    <svg
                      className="h-4 w-4 text-gray-400"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                      strokeWidth={2}
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        d="M19 9l-7 7-7-7"
                      />
                    </svg>
                  </div>
                </div>
                {errors.roleId && (
                  <p className="mt-1 text-xs text-red-600">{errors.roleId}</p>
                )}
                <p className="mt-1 text-xs text-gray-500">
                  {roleId
                    ? "Rolle kann später geändert werden"
                    : "Bitte wähle eine Rolle aus"}
                </p>
              </div>
            )}

            {/* RFID Tag UI intentionally hidden by default */}
            {showRFID && (
              <div>
                <label
                  htmlFor="tagId"
                  className="mb-1 block text-sm font-medium text-gray-500"
                >
                  RFID-Karte (Funktion nicht verfügbar)
                </label>
                <div className="relative">
                  <select
                    id="tagId"
                    value=""
                    onChange={(e) => setTagId(e.target.value)}
                    className="w-full cursor-not-allowed rounded-lg border border-gray-200 bg-gray-50 px-4 py-2 text-gray-500"
                    disabled={true}
                  >
                    <option value="">RFID-Funktion deaktiviert</option>
                  </select>
                  <p className="mt-1 text-xs text-gray-500">
                    Die RFID-Kartenzuweisung ist derzeit nicht verfügbar
                  </p>
                </div>
              </div>
            )}

            {/* Password - only for new teachers */}
            {!initialData.id && (
              <>
                <div>
                  <label
                    htmlFor="password"
                    className="mb-1 block text-xs font-medium text-gray-700"
                  >
                    Passwort <span className="text-red-500">*</span>
                  </label>
                  <input
                    type="password"
                    id="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    className={`w-full rounded-lg border ${
                      errors.password
                        ? "border-red-300 bg-red-50"
                        : "border-gray-200 bg-white focus:border-[#F78C10] focus:ring-1 focus:ring-[#F78C10]"
                    } px-3 py-2 text-sm transition-colors`}
                    disabled={isLoading}
                  />
                  {errors.password && (
                    <p className="mt-1 text-xs text-red-600">
                      {errors.password}
                    </p>
                  )}
                </div>

                <div>
                  <label
                    htmlFor="confirmPassword"
                    className="mb-1 block text-xs font-medium text-gray-700"
                  >
                    Passwort bestätigen <span className="text-red-500">*</span>
                  </label>
                  <input
                    type="password"
                    id="confirmPassword"
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                    className={`w-full rounded-lg border ${
                      errors.confirmPassword
                        ? "border-red-300 bg-red-50"
                        : "border-gray-200 bg-white focus:border-[#F78C10] focus:ring-1 focus:ring-[#F78C10]"
                    } px-3 py-2 text-sm transition-colors`}
                    disabled={isLoading}
                  />
                  {errors.confirmPassword && (
                    <p className="mt-1 text-xs text-red-600">
                      {errors.confirmPassword}
                    </p>
                  )}
                </div>
              </>
            )}
          </div>
        </div>

        {/* Professional Information Section */}
        <div className="rounded-xl border border-gray-100 bg-orange-50/30 p-3 md:p-4">
          <h4 className="mb-3 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-4 md:text-sm">
            <svg
              className="h-3.5 w-3.5 text-orange-600 md:h-4 md:w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M21 13.255A23.931 23.931 0 0112 15c-3.183 0-6.22-.62-9-1.745M16 6V4a2 2 0 00-2-2h-4a2 2 0 00-2 2v2m4 6h.01M5 20h14a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
              />
            </svg>
            Berufliche Informationen
          </h4>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2 md:gap-4">
            {/* Position */}
            <div>
              <label
                htmlFor="role"
                className="mb-1 block text-xs font-medium text-gray-700"
              >
                Position
              </label>
              <div className="relative">
                <select
                  id="role"
                  value={role}
                  onChange={(e) => setRole(e.target.value)}
                  className="w-full appearance-none rounded-lg border border-gray-200 bg-white px-3 py-2 pr-10 text-sm transition-colors focus:border-[#F78C10] focus:ring-1 focus:ring-[#F78C10]"
                  disabled={isLoading}
                >
                  <option value="">Position auswählen</option>
                  <option value="Pädagogische Fachkraft">
                    Pädagogische Fachkraft
                  </option>
                  <option value="OGS-Büro">OGS-Büro</option>
                  <option value="Extern">Extern</option>
                </select>
                <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3">
                  <svg
                    className="h-4 w-4 text-gray-400"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    strokeWidth={2}
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      d="M19 9l-7 7-7-7"
                    />
                  </svg>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Form Actions */}
        <div className="sticky bottom-0 -mx-4 mt-4 -mb-4 flex gap-2 border-t border-gray-100 bg-white/95 px-4 pt-3 pb-3 backdrop-blur-sm md:-mx-6 md:mt-6 md:-mb-6 md:gap-3 md:px-6 md:pt-4 md:pb-4">
          <button
            type="button"
            onClick={onCancelAction}
            disabled={isLoading}
            className="flex-1 rounded-lg border border-gray-300 px-3 py-2 text-xs font-medium text-gray-700 transition-all duration-200 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 md:px-4 md:text-sm md:hover:scale-105"
          >
            <span className="flex items-center justify-center gap-2">
              <svg
                className="h-4 w-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
              Abbrechen
            </span>
          </button>
          <button
            type="submit"
            disabled={isLoading}
            className="flex-1 rounded-lg bg-gray-900 px-3 py-2 text-xs font-medium text-white transition-all duration-200 hover:bg-gray-700 hover:shadow-lg active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 md:px-4 md:text-sm md:hover:scale-105"
          >
            {isLoading ? (
              <span className="flex items-center justify-center gap-2">
                <svg
                  className="h-4 w-4 animate-spin text-white"
                  fill="none"
                  viewBox="0 0 24 24"
                >
                  <circle
                    className="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    strokeWidth="4"
                  ></circle>
                  <path
                    className="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                  ></path>
                </svg>
                Wird gespeichert...
              </span>
            ) : (
              <span className="flex items-center justify-center gap-2">
                <svg
                  className="h-4 w-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d={
                      submitLabel === "Erstellen"
                        ? "M12 4v16m8-8H4"
                        : "M5 13l4 4L19 7"
                    }
                  />
                </svg>
                {submitLabel}
              </span>
            )}
          </button>
        </div>
      </form>
    </div>
  );
}
