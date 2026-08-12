import { useState, useEffect, useRef } from "react";
import { Check, Loader2, Plus, X } from "lucide-react";
import { BriefcaseIcon, UserIcon } from "@phosphor-icons/react";
import type { Teacher } from "@/lib/teacher-api";
import { authService } from "@/lib/auth-service";
import { toAssignableRoleOptions, type RoleOption } from "@/lib/auth-helpers";
import { CustomSelect } from "~/components/ui/custom-select";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { createLogger } from "~/lib/logger";
import { useScrollToError } from "~/lib/hooks/use-scroll-to-error";

const logger = createLogger({ component: "TeacherForm" });
const EMPTY_RFID_CARDS: ReadonlyArray<{ id: string; label: string }> = [];
const EMPTY_POSITIONS: readonly string[] = [];

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
  // Existing positions for autocomplete suggestions
  readonly existingPositions?: readonly string[];
}

export function TeacherForm({
  initialData,
  onSubmitAction,
  onCancelAction,
  isLoading,
  formTitle = "Details des Personals",
  submitLabel = "Speichern",
  rfidCards: _rfidCards = EMPTY_RFID_CARDS,
  wrapInCard = true,
  showRFID = false,
  existingPositions = EMPTY_POSITIONS,
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
  const errorRef = useScrollToError(
    submitError ?? (Object.keys(errors).length > 0 ? "validation" : null),
  );

  // Store a reference to track when we need to reset the form
  const prevIdRef = useRef(initialData?.id);

  // Position (Teacher.Role) lebt auf users.teachers. Ohne Betreuungsprofil
  // kann der Wert nirgends gespeichert werden — UpdateStaffWithTeacher
  // schreibt die Teacher-Felder bei is_teacher=false gar nicht erst. Das
  // Feld auszublenden ist ehrlicher als ein Speichern, das kommentarlos
  // verworfen wird. Edit: entscheidet der geladene Datensatz mit derselben
  // Auflösung wie buildStaffUpdateData (is_teacher, Fallback teacher_id) —
  // fehlt beides, wird versteckt, denn genau dann würde der Update-Pfad
  // den Wert verwerfen. Anlegen: die gewählte System-Rolle.
  const selectedRoleIsLehrkraft =
    roles.find((option) => option.id === roleId)?.systemName === "lehrkraft";
  const hidePosition = initialData.id
    ? !(initialData.is_teacher ?? Boolean(initialData.teacher_id))
    : selectedRoleIsLehrkraft;

  // Fetch roles on mount (only for new teachers)
  useEffect(() => {
    if (initialData.id) return; // Skip for editing existing teachers

    let cancelled = false;
    async function fetchRoles() {
      try {
        setIsLoadingRoles(true);
        const roleList = await authService.getRoles();
        if (cancelled) return;

        setRoles(toAssignableRoleOptions(roleList));
      } catch (err) {
        logger.error("failed to load roles", {
          error: err instanceof Error ? err.message : String(err),
        });
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

  // Helper to validate password strength
  const validatePassword = (pwd: string): string | null => {
    if (!pwd) return "Passwort ist erforderlich";
    if (pwd.length < 8) return "Passwort muss mindestens 8 Zeichen lang sein";
    if (!/[A-Z]/.test(pwd))
      return "Passwort muss mindestens einen Großbuchstaben enthalten";
    if (!/[a-z]/.test(pwd))
      return "Passwort muss mindestens einen Kleinbuchstaben enthalten";
    if (!/\d/.test(pwd)) return "Passwort muss mindestens eine Zahl enthalten";
    if (!/[^a-zA-Z0-9]/.test(pwd))
      return "Passwort muss mindestens ein Sonderzeichen enthalten";
    return null;
  };

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
      } else if (
        email.length > 254 ||
        !/^[a-zA-Z0-9_+&*-]+(?:\.[a-zA-Z0-9_+&*-]+)*@(?:[a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}$/.test(
          email,
        )
      ) {
        newErrors.email = "Ungültige E-Mail-Adresse";
      }

      if (!roleId || roleId <= 0) {
        newErrors.roleId = "Bitte wähle eine Rolle aus";
      }

      const passwordError = validatePassword(password);
      if (passwordError) {
        newErrors.password = passwordError;
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
      // Lehrkraft (#1772) bekommt KEIN Betreuungsprofil (users.teachers) —
      // dieselbe Invariante wie Einladungs-Flow und Operator-Provisioning.
      // Prepare data for submission
      const formData: Partial<Teacher> & {
        password?: string;
        role_id?: number;
      } = {
        first_name: firstName.trim(),
        last_name: lastName.trim(),
        email: email.trim() || undefined,
        // Ohne Betreuungsprofil gibt es kein Position-Feld — den Wert dann
        // auch nicht mitschicken, das Backend würde ihn ohnehin verwerfen.
        ...(!hidePosition && { role: role.trim() || null }),
        tag_id: tagId || null, // Use the TagID directly
        // Preserve existing IDs when editing
        ...(initialData.id && { id: initialData.id }),
        ...(initialData.person_id && { person_id: initialData.person_id }),
        // is_teacher nur beim Anlegen mitschicken: im Edit-Modus gibt es
        // keine Rollenauswahl (roleId bleibt undefined), der Flag wäre also
        // immer true und würde einer Lehrkraft beim Speichern eines
        // Tippfehlers ein Betreuungsprofil anlegen. updateTeacher erhält
        // stattdessen den bestehenden Zustand.
        ...(!initialData.id && { is_teacher: !selectedRoleIsLehrkraft }),
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
      const errorMsg = err instanceof Error ? err.message : String(err);
      logger.error("failed to submit form", { error: errorMsg });
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
        <div
          ref={errorRef}
          className="border-moto-red/20 bg-moto-red-soft text-moto-red-strong mb-4 rounded-lg border p-3 text-xs md:mb-6 md:p-4 md:text-sm"
        >
          {submitError}
        </div>
      )}

      <form
        onSubmit={handleSubmit}
        noValidate
        className="space-y-4 md:space-y-6"
      >
        {/* Personal Information Section */}
        <div className="bg-moto-orange/10 rounded-xl border border-gray-100 p-3 md:p-4">
          <h4 className="mb-3 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-4 md:text-sm">
            <span className="flex h-3.5 w-3.5 shrink-0 md:h-4 md:w-4">
              <MotoDuotoneIcon icon={UserIcon} tone="neutral" size="100%" />
            </span>
            Persönliche Informationen
          </h4>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2 md:gap-4">
            {/* First Name */}
            <div>
              <label
                htmlFor="firstName"
                className="mb-1 block text-xs font-medium text-gray-700"
              >
                Vorname <span className="text-moto-red">*</span>
              </label>
              <input
                type="text"
                id="firstName"
                name="firstName"
                value={firstName}
                onChange={(e) => setFirstName(e.target.value)}
                className={`w-full rounded-lg border ${
                  errors.firstName
                    ? "border-moto-red/40 bg-moto-red-soft"
                    : "focus:border-moto-orange focus:ring-moto-orange border-gray-200 bg-white focus:ring-1"
                } px-3 py-2 text-sm transition-colors`}
                disabled={isLoading}
                autoComplete="given-name"
                maxLength={255}
              />
              {errors.firstName && (
                <p className="text-moto-red-strong mt-1 text-xs">
                  {errors.firstName}
                </p>
              )}
            </div>

            {/* Last Name */}
            <div>
              <label
                htmlFor="lastName"
                className="mb-1 block text-xs font-medium text-gray-700"
              >
                Nachname <span className="text-moto-red">*</span>
              </label>
              <input
                type="text"
                id="lastName"
                name="lastName"
                value={lastName}
                onChange={(e) => setLastName(e.target.value)}
                className={`w-full rounded-lg border ${
                  errors.lastName
                    ? "border-moto-red/40 bg-moto-red-soft"
                    : "focus:border-moto-orange focus:ring-moto-orange border-gray-200 bg-white focus:ring-1"
                } px-3 py-2 text-sm transition-colors`}
                disabled={isLoading}
                autoComplete="family-name"
                maxLength={255}
              />
              {errors.lastName && (
                <p className="text-moto-red-strong mt-1 text-xs">
                  {errors.lastName}
                </p>
              )}
            </div>

            {/* Email - only for new teachers */}
            {!initialData.id && (
              <div>
                <label
                  htmlFor="email"
                  className="mb-1 block text-xs font-medium text-gray-700"
                >
                  E-Mail <span className="text-moto-red">*</span>
                </label>
                <input
                  type="email"
                  id="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className={`w-full rounded-lg border ${
                    errors.email
                      ? "border-moto-red/40 bg-moto-red-soft"
                      : "focus:border-moto-orange focus:ring-moto-orange border-gray-200 bg-white focus:ring-1"
                  } px-3 py-2 text-sm transition-colors`}
                  disabled={isLoading}
                  maxLength={255}
                />
                {errors.email && (
                  <p className="text-moto-red-strong mt-1 text-xs">
                    {errors.email}
                  </p>
                )}
              </div>
            )}

            {/* Role Selection - only for new teachers */}
            {!initialData.id && (
              <div>
                <label
                  id="role-select-label"
                  htmlFor="role-select"
                  className="mb-1 block text-xs font-medium text-gray-700"
                >
                  System-Rolle <span className="text-moto-red">*</span>
                </label>
                <CustomSelect
                  id="role-select"
                  ariaLabelledBy="role-select-label"
                  value={roleId === undefined ? "" : String(roleId)}
                  onChange={(next) =>
                    setRoleId(next === "" ? undefined : Number(next))
                  }
                  options={roles.map((role) => ({
                    value: String(role.id),
                    label: role.name,
                  }))}
                  placeholder={
                    isLoadingRoles ? "Lade Rollen..." : "Rolle auswählen..."
                  }
                  disabled={isLoading || isLoadingRoles}
                  invalid={Boolean(errors.roleId)}
                />
                {errors.roleId && (
                  <p className="text-moto-red-strong mt-1 text-xs">
                    {errors.roleId}
                  </p>
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
                  id="tagId-label"
                  htmlFor="tagId"
                  className="mb-1 block text-sm font-medium text-gray-500"
                >
                  RFID-Karte (Funktion nicht verfügbar)
                </label>
                <CustomSelect
                  id="tagId"
                  ariaLabelledBy="tagId-label"
                  value=""
                  onChange={(next) => setTagId(next)}
                  options={[{ value: "", label: "RFID-Funktion deaktiviert" }]}
                  disabled={true}
                />
                <p className="mt-1 text-xs text-gray-500">
                  Die RFID-Kartenzuweisung ist derzeit nicht verfügbar
                </p>
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
                    Passwort <span className="text-moto-red">*</span>
                  </label>
                  <input
                    type="password"
                    id="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    className={`w-full rounded-lg border ${
                      errors.password
                        ? "border-moto-red/40 bg-moto-red-soft"
                        : "focus:border-moto-orange focus:ring-moto-orange border-gray-200 bg-white focus:ring-1"
                    } px-3 py-2 text-sm transition-colors`}
                    disabled={isLoading}
                  />
                  {errors.password && (
                    <p className="text-moto-red-strong mt-1 text-xs">
                      {errors.password}
                    </p>
                  )}
                </div>

                <div>
                  <label
                    htmlFor="confirmPassword"
                    className="mb-1 block text-xs font-medium text-gray-700"
                  >
                    Passwort bestätigen <span className="text-moto-red">*</span>
                  </label>
                  <input
                    type="password"
                    id="confirmPassword"
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                    className={`w-full rounded-lg border ${
                      errors.confirmPassword
                        ? "border-moto-red/40 bg-moto-red-soft"
                        : "focus:border-moto-orange focus:ring-moto-orange border-gray-200 bg-white focus:ring-1"
                    } px-3 py-2 text-sm transition-colors`}
                    disabled={isLoading}
                  />
                  {errors.confirmPassword && (
                    <p className="text-moto-red-strong mt-1 text-xs">
                      {errors.confirmPassword}
                    </p>
                  )}
                </div>
              </>
            )}
          </div>
        </div>

        {/* Professional Information Section — nur mit Betreuungsprofil:
            Position lebt auf users.teachers und kann für Lehrkräfte
            nirgends gespeichert werden. */}
        {hidePosition ? null : (
          <div className="bg-moto-orange/10 rounded-xl border border-gray-100 p-3 md:p-4">
            <h4 className="mb-3 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-4 md:text-sm">
              <span className="flex h-3.5 w-3.5 shrink-0 md:h-4 md:w-4">
                <MotoDuotoneIcon
                  icon={BriefcaseIcon}
                  tone="neutral"
                  size="100%"
                />
              </span>
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
                <input
                  type="text"
                  id="role"
                  list="teacher-position-suggestions"
                  value={role}
                  onChange={(e) => setRole(e.target.value)}
                  className="focus:border-moto-orange focus:ring-moto-orange w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm transition-colors focus:ring-1"
                  placeholder="z.B. Pädagogische Fachkraft, OGS-Büro"
                  disabled={isLoading}
                />
                {existingPositions.length > 0 && (
                  <datalist id="teacher-position-suggestions">
                    {existingPositions.map((pos) => (
                      <option key={pos} value={pos} />
                    ))}
                  </datalist>
                )}
              </div>
            </div>
          </div>
        )}

        {/* Form Actions */}
        <div className="sticky bottom-0 -mx-4 mt-4 -mb-4 flex gap-2 border-t border-gray-100 bg-white/95 px-4 pt-3 pb-3 backdrop-blur-sm md:-mx-6 md:mt-6 md:-mb-6 md:gap-3 md:px-6 md:pt-4 md:pb-4">
          <button
            type="button"
            onClick={onCancelAction}
            disabled={isLoading}
            className="flex-1 rounded-lg border border-gray-300 px-3 py-2 text-xs font-medium text-gray-700 transition-all duration-200 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 md:px-4 md:text-sm md:hover:scale-105"
          >
            <span className="flex items-center justify-center gap-2">
              <X className="h-4 w-4" aria-hidden />
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
                <Loader2
                  className="h-4 w-4 animate-spin text-white"
                  aria-hidden
                />
                Wird gespeichert...
              </span>
            ) : (
              <span className="flex items-center justify-center gap-2">
                {submitLabel === "Erstellen" ? (
                  <Plus className="h-4 w-4" aria-hidden />
                ) : (
                  <Check className="h-4 w-4" aria-hidden />
                )}
                {submitLabel}
              </span>
            )}
          </button>
        </div>
      </form>
    </div>
  );
}
