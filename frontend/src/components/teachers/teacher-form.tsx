import { useState, useEffect } from "react";
import type { Teacher } from "@/lib/teacher-api";

interface TeacherFormProps {
    initialData: Partial<Teacher>;
    onSubmitAction: (data: Partial<Teacher>) => Promise<void>;
    onCancelAction: () => void;
    isLoading: boolean;
    formTitle?: string;
    submitLabel?: string;
    rfidCards?: Array<{ id: string; label: string }>;
}

export default function TeacherForm({
                                        initialData,
                                        onSubmitAction,
                                        onCancelAction,
                                        isLoading,
                                        formTitle = "Lehrerdetails",
                                        submitLabel = "Speichern",
                                        rfidCards = [],
                                    }: TeacherFormProps) {
    // Form state
    const [firstName, setFirstName] = useState(initialData.first_name ?? "");
    const [lastName, setLastName] = useState(initialData.last_name ?? "");
    const [email, setEmail] = useState(initialData.email ?? "");
    const [password, setPassword] = useState("");
    const [confirmPassword, setConfirmPassword] = useState("");
    const [specialization, setSpecialization] = useState(
        initialData.specialization ?? ""
    );
    const [role, setRole] = useState(initialData.role ?? "");
    const [qualifications, setQualifications] = useState(
        initialData.qualifications ?? ""
    );
    const [tagId, setTagId] = useState(initialData.tag_id ?? "");
    const [staffNotes, setStaffNotes] = useState(initialData.staff_notes ?? "");

    // Form validation
    const [errors, setErrors] = useState<Record<string, string>>({});
    const [submitError, setSubmitError] = useState<string | null>(null);

    // Update form when initialData changes
    useEffect(() => {
        if (initialData) {
            setFirstName(initialData.first_name ?? "");
            setLastName(initialData.last_name ?? "");
            setEmail(initialData.email ?? "");
            setSpecialization(initialData.specialization ?? "");
            setRole(initialData.role ?? "");
            setQualifications(initialData.qualifications ?? "");
            setTagId(initialData.tag_id ?? "");
            setStaffNotes(initialData.staff_notes ?? "");
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

        // Email validation only for new teachers
        if (!initialData.id) {
            if (!email.trim()) {
                newErrors.email = "E-Mail ist erforderlich";
            } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
                newErrors.email = "Ungültige E-Mail-Adresse";
            }

            if (!password) {
                newErrors.password = "Passwort ist erforderlich";
            } else if (password.length < 8) {
                newErrors.password = "Passwort muss mindestens 8 Zeichen lang sein";
            } else if (!/[A-Z]/.test(password)) {
                newErrors.password = "Passwort muss mindestens einen Großbuchstaben enthalten";
            } else if (!/[a-z]/.test(password)) {
                newErrors.password = "Passwort muss mindestens einen Kleinbuchstaben enthalten";
            } else if (!/[0-9]/.test(password)) {
                newErrors.password = "Passwort muss mindestens eine Zahl enthalten";
            } else if (!/[^a-zA-Z0-9]/.test(password)) {
                newErrors.password = "Passwort muss mindestens ein Sonderzeichen enthalten";
            }

            if (!confirmPassword) {
                newErrors.confirmPassword = "Passwortbestätigung ist erforderlich";
            } else if (password !== confirmPassword) {
                newErrors.confirmPassword = "Passwörter stimmen nicht überein";
            }
        }

        if (!specialization.trim()) {
            newErrors.specialization = "Fachgebiet ist erforderlich";
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
            const formData: Partial<Teacher> = {
                first_name: firstName.trim(),
                last_name: lastName.trim(),
                email: email.trim() || undefined,
                specialization: specialization.trim(),
                role: role.trim() || null,
                qualifications: qualifications.trim() || null,  
                tag_id: tagId || null,
                staff_notes: staffNotes.trim() || null,
            };

            // Include password for new teachers (it's always required now)
            if (!initialData.id) {
                (formData as any).password = password;
            }

            // Submit the form
            await onSubmitAction(formData);
        } catch (err) {
            console.error("Error submitting form:", err);
            setSubmitError(
                "Es ist ein Fehler aufgetreten. Bitte versuchen Sie es später erneut."
            );
        }
    };

    return (
        <div className="rounded-lg border border-gray-100 bg-white p-6 shadow-sm">
            <h3 className="mb-6 text-lg font-semibold text-gray-800">{formTitle}</h3>

            {submitError && (
                <div className="mb-6 rounded-lg border border-red-200 bg-red-50 p-4 text-red-800">
                    {submitError}
                </div>
            )}

            <form onSubmit={handleSubmit} className="space-y-6">
                {/* Personal Information Section */}
                <div>
                    <h4 className="mb-4 text-md font-medium text-gray-700">Persönliche Informationen</h4>
                    <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                        {/* First Name */}
                        <div>
                            <label
                                htmlFor="firstName"
                                className="mb-1 block text-sm font-medium text-gray-700"
                            >
                                Vorname <span className="text-red-500">*</span>
                            </label>
                            <input
                                type="text"
                                id="firstName"
                                value={firstName}
                                onChange={(e) => setFirstName(e.target.value)}
                                className={`w-full rounded-lg border ${
                                    errors.firstName ? "border-red-300" : "border-gray-300"
                                } px-4 py-2 transition-colors focus:ring-2 focus:ring-blue-500 focus:outline-none`}
                                disabled={isLoading}
                            />
                            {errors.firstName && (
                                <p className="mt-1 text-sm text-red-600">{errors.firstName}</p>
                            )}
                        </div>

                        {/* Last Name */}
                        <div>
                            <label
                                htmlFor="lastName"
                                className="mb-1 block text-sm font-medium text-gray-700"
                            >
                                Nachname <span className="text-red-500">*</span>
                            </label>
                            <input
                                type="text"
                                id="lastName"
                                value={lastName}
                                onChange={(e) => setLastName(e.target.value)}
                                className={`w-full rounded-lg border ${
                                    errors.lastName ? "border-red-300" : "border-gray-300"
                                } px-4 py-2 transition-colors focus:ring-2 focus:ring-blue-500 focus:outline-none`}
                                disabled={isLoading}
                            />
                            {errors.lastName && (
                                <p className="mt-1 text-sm text-red-600">{errors.lastName}</p>
                            )}
                        </div>

                        {/* Email - only for new teachers */}
                        {!initialData.id && (
                            <div>
                                <label
                                    htmlFor="email"
                                    className="mb-1 block text-sm font-medium text-gray-700"
                                >
                                    E-Mail <span className="text-red-500">*</span>
                                </label>
                                <input
                                    type="email"
                                    id="email"
                                    value={email}
                                    onChange={(e) => setEmail(e.target.value)}
                                    className={`w-full rounded-lg border ${
                                        errors.email ? "border-red-300" : "border-gray-300"
                                    } px-4 py-2 transition-colors focus:ring-2 focus:ring-blue-500 focus:outline-none`}
                                    disabled={isLoading}
                                />
                                {errors.email && (
                                    <p className="mt-1 text-sm text-red-600">{errors.email}</p>
                                )}
                            </div>
                        )}

                        {/* RFID Tag */}
                        <div>
                            <label
                                htmlFor="tagId"
                                className="mb-1 block text-sm font-medium text-gray-700"
                            >
                                RFID-Karte
                            </label>
                            <select
                                id="tagId"
                                value={tagId}
                                onChange={(e) => setTagId(e.target.value)}
                                className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-colors focus:ring-2 focus:ring-blue-500 focus:outline-none"
                                disabled={isLoading}
                            >
                                <option value="">Keine RFID-Karte</option>
                                {rfidCards.map((card) => (
                                    <option key={card.id} value={card.id}>
                                        {card.label}
                                    </option>
                                ))}
                            </select>
                        </div>

                        {/* Password - only for new teachers */}
                        {!initialData.id && (
                            <>
                                <div>
                                    <label
                                        htmlFor="password"
                                        className="mb-1 block text-sm font-medium text-gray-700"
                                    >
                                        Passwort <span className="text-red-500">*</span>
                                    </label>
                                    <input
                                        type="password"
                                        id="password"
                                        value={password}
                                        onChange={(e) => setPassword(e.target.value)}
                                        className={`w-full rounded-lg border ${
                                            errors.password ? "border-red-300" : "border-gray-300"
                                        } px-4 py-2 transition-colors focus:ring-2 focus:ring-blue-500 focus:outline-none`}
                                        disabled={isLoading}
                                    />
                                    {errors.password && (
                                        <p className="mt-1 text-sm text-red-600">{errors.password}</p>
                                    )}
                                </div>

                                <div>
                                    <label
                                        htmlFor="confirmPassword"
                                        className="mb-1 block text-sm font-medium text-gray-700"
                                    >
                                        Passwort bestätigen <span className="text-red-500">*</span>
                                    </label>
                                    <input
                                        type="password"
                                        id="confirmPassword"
                                        value={confirmPassword}
                                        onChange={(e) => setConfirmPassword(e.target.value)}
                                        className={`w-full rounded-lg border ${
                                            errors.confirmPassword ? "border-red-300" : "border-gray-300"
                                        } px-4 py-2 transition-colors focus:ring-2 focus:ring-blue-500 focus:outline-none`}
                                        disabled={isLoading}
                                    />
                                    {errors.confirmPassword && (
                                        <p className="mt-1 text-sm text-red-600">{errors.confirmPassword}</p>
                                    )}
                                </div>
                            </>
                        )}
                    </div>
                </div>

                {/* Professional Information Section */}
                <div>
                    <h4 className="mb-4 text-md font-medium text-gray-700">Berufliche Informationen</h4>
                    <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                        {/* Specialization */}
                        <div>
                            <label
                                htmlFor="specialization"
                                className="mb-1 block text-sm font-medium text-gray-700"
                            >
                                Fachgebiet <span className="text-red-500">*</span>
                            </label>
                            <input
                                type="text"
                                id="specialization"
                                value={specialization}
                                onChange={(e) => setSpecialization(e.target.value)}
                                className={`w-full rounded-lg border ${
                                    errors.specialization ? "border-red-300" : "border-gray-300"
                                } px-4 py-2 transition-colors focus:ring-2 focus:ring-blue-500 focus:outline-none`}
                                disabled={isLoading}
                            />
                            {errors.specialization && (
                                <p className="mt-1 text-sm text-red-600">{errors.specialization}</p>
                            )}
                        </div>

                        {/* Role */}
                        <div>
                            <label
                                htmlFor="role"
                                className="mb-1 block text-sm font-medium text-gray-700"
                            >
                                Rolle
                            </label>
                            <input
                                type="text"
                                id="role"
                                value={role}
                                onChange={(e) => setRole(e.target.value)}
                                className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-colors focus:ring-2 focus:ring-blue-500 focus:outline-none"
                                disabled={isLoading}
                            />
                        </div>

                        {/* Qualifications */}
                        <div className="md:col-span-2">
                            <label
                                htmlFor="qualifications"
                                className="mb-1 block text-sm font-medium text-gray-700"
                            >
                                Qualifikationen
                            </label>
                            <input
                                type="text"
                                id="qualifications"
                                value={qualifications}
                                onChange={(e) => setQualifications(e.target.value)}
                                className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-colors focus:ring-2 focus:ring-blue-500 focus:outline-none"
                                disabled={isLoading}
                            />
                        </div>
                    </div>
                </div>

                {/* Additional Information Section */}
                <div>
                    <h4 className="mb-4 text-md font-medium text-gray-700">Zusätzliche Informationen</h4>
                    <div>
                        <label
                            htmlFor="staffNotes"
                            className="mb-1 block text-sm font-medium text-gray-700"
                        >
                            Notizen
                        </label>
                        <textarea
                            id="staffNotes"
                            value={staffNotes}
                            onChange={(e) => setStaffNotes(e.target.value)}
                            rows={4}
                            className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-colors focus:ring-2 focus:ring-blue-500 focus:outline-none"
                            disabled={isLoading}
                        />
                    </div>
                </div>

                {/* Form Actions */}
                <div className="flex flex-col gap-3 pt-4 sm:flex-row-reverse">
                    <button
                        type="submit"
                        className="rounded-lg bg-blue-600 px-4 py-2 text-white transition-colors hover:bg-blue-700 disabled:bg-blue-300"
                        disabled={isLoading}
                    >
                        {isLoading ? "Wird gespeichert..." : submitLabel}
                    </button>
                    <button
                        type="button"
                        onClick={onCancelAction}
                        className="rounded-lg border border-gray-300 px-4 py-2 text-gray-700 transition-colors hover:bg-gray-50 disabled:opacity-50"
                        disabled={isLoading}
                    >
                        Abbrechen
                    </button>
                </div>
            </form>
        </div>
    );
}