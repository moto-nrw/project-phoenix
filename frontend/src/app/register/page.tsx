"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { env } from "~/env";
import {
    Card,
    CardHeader,
    CardContent,
    CardFooter,
    Input,
    Button,
    Alert,
    Link,
    HelpButton,
} from "~/components/ui";


export default function RegisterPage() {
    const [formData, setFormData] = useState({
        email: "",
        username: "",
        name: "",
        password: "",
        confirmPassword: "",
    });
    const [errors, setErrors] = useState<Record<string, string>>({});
    const [serverError, setServerError] = useState("");
    const [success, setSuccess] = useState("");
    const [isLoading, setIsLoading] = useState(false);
    const router = useRouter();

    const validateForm = () => {
        const newErrors: Record<string, string> = {};

        if (!formData.email) {
            newErrors.email = "E-Mail ist erforderlich";
        } else if (!/\S+@\S+\.\S+/.test(formData.email)) {
            newErrors.email = "E-Mail ist ungültig";
        }

        if (!formData.username) {
            newErrors.username = "Benutzername ist erforderlich";
        } else if (!/^[a-zA-Z0-9]{3,30}$/.test(formData.username)) {
            newErrors.username = "Benutzername muss 3-30 alphanumerische Zeichen enthalten";
        }

        if (!formData.name) {
            newErrors.name = "Name ist erforderlich";
        }

        if (!formData.password) {
            newErrors.password = "Passwort ist erforderlich";
        } else if (formData.password.length < 8) {
            newErrors.password = "Passwort muss mindestens 8 Zeichen lang sein";
        } else if (!/[A-Z]/.test(formData.password)) {
            newErrors.password = "Passwort muss einen Großbuchstaben enthalten";
        } else if (!/[a-z]/.test(formData.password)) {
            newErrors.password = "Passwort muss einen Kleinbuchstaben enthalten";
        } else if (!/[0-9]/.test(formData.password)) {
            newErrors.password = "Passwort muss eine Zahl enthalten";
        } else if (!/[^a-zA-Z0-9]/.test(formData.password)) {
            newErrors.password = "Passwort muss ein Sonderzeichen enthalten";
        }

        if (formData.password !== formData.confirmPassword) {
            newErrors.confirmPassword = "Passwörter stimmen nicht überein";
        }

        setErrors(newErrors);
        return Object.keys(newErrors).length === 0;
    };

    const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        const { name, value } = e.target;
        setFormData((prev) => ({ ...prev, [name]: value }));
    };

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setServerError("");
        setSuccess("");

        if (!validateForm()) return;

        setIsLoading(true);

        try {
            // Use relative URL when in the browser to avoid CORS issues with Docker hostnames
            const apiUrl =
                typeof window !== "undefined"
                    ? "/api/auth/register"
                    : `${env.NEXT_PUBLIC_API_URL}/auth/register`;

            const response = await fetch(apiUrl, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({
                    email: formData.email,
                    username: formData.username,
                    name: formData.name,
                    password: formData.password,
                    confirm_password: formData.confirmPassword,
                }),
            });

            const responseData = (await response.json()) as {
                error?: string;
                message?: string;
                status?: string;
            };

            if (!response.ok) {
                // Extract the specific error message from the backend response
                let errorMessage = "Registrierung fehlgeschlagen";

                if (responseData.error) {
                    // Handle specific error cases from the backend
                    switch (responseData.error) {
                        case "email already exists":
                            errorMessage = "Ein Konto mit dieser E-Mail-Adresse existiert bereits";
                            break;
                        case "username already exists":
                            errorMessage = "Dieser Benutzername ist bereits vergeben";
                            break;
                        case "password is too short (minimum 8 characters)":
                            errorMessage = "Passwort muss mindestens 8 Zeichen lang sein";
                            break;
                        case "password must contain at least one uppercase letter":
                            errorMessage = "Passwort muss einen Großbuchstaben enthalten";
                            break;
                        case "password must contain at least one lowercase letter":
                            errorMessage = "Passwort muss einen Kleinbuchstaben enthalten";
                            break;
                        case "password must contain at least one number":
                            errorMessage = "Passwort muss eine Zahl enthalten";
                            break;
                        case "password must contain at least one special character":
                            errorMessage = "Passwort muss ein Sonderzeichen enthalten";
                            break;
                        case "passwords do not match":
                            errorMessage = "Passwörter stimmen nicht überein";
                            break;
                        default:
                            // If we have a specific error message from the backend, use it
                            errorMessage = responseData.error;
                    }
                } else if (responseData.message) {
                    errorMessage = responseData.message;
                } else if (responseData.status) {
                    errorMessage = responseData.status;
                }

                setServerError(errorMessage);
            } else {
                setSuccess("Konto erfolgreich erstellt! Weiterleitung zur Anmeldung...");
                setTimeout(() => {
                    router.push("/login");
                }, 2000);
            }
        } catch (error) {
            setServerError("Ein Fehler ist bei der Registrierung aufgetreten");
            console.error(error);
        } finally {
            setIsLoading(false);
        }
    };

    return (
        <div className="flex min-h-screen flex-col items-center justify-center p-4">
            <Card>
                <div className="flex justify-end mb-4">
                    <HelpButton
                        title="Registrierungsformular Hilfe"
                        content={
                            <div className="space-y-4">
                                <div>
                                    <h3 className="font-semibold">E-Mail-Adresse</h3>
                                    <p>Geben Sie eine gültige E-Mail-Adresse ein. </p>
                                </div>

                                <div>
                                    <h3 className="font-semibold">Benutzername</h3>
                                    <p>Erstellen Sie einen eindeutigen Benutzernamen für Ihr Konto:</p>
                                    <ul className="mt-1 list-disc pl-4">
                                        <li>3 bis 30 Zeichen lang</li>
                                        <li>Nur Buchstaben und Zahlen (keine Sonderzeichen)</li>
                                    </ul>
                                </div>

                                <div>
                                    <h3 className="font-semibold">Passwort-Anforderungen</h3>
                                    <p>Erstellen Sie ein sicheres Passwort mit folgenden Anforderungen:</p>
                                    <ul className="mt-1 list-disc pl-4">
                                        <li>Mindestens 8 Zeichen lang</li>
                                        <li>Mindestens 1 Großbuchstabe (A-Z)</li>
                                        <li>Mindestens 1 Kleinbuchstabe (a-z)</li>
                                        <li>Mindestens 1 Zahl (0-9)</li>
                                        <li>Mindestens 1 Sonderzeichen (!@#$%^&*)</li>
                                    </ul>
                                </div>
                            </div>
                        }
                    />
                </div>
                <CardHeader
                    title="Konto erstellen"
                    description="Geben Sie Ihre Daten ein, um ein neues Konto zu erstellen"
                />

                <CardContent>


                    <form onSubmit={handleSubmit} className="space-y-6">
                        {serverError && <Alert type="error" message={serverError} />}
                        {success && <Alert type="success" message={success} />}

                        <div className="space-y-4">
                            <Input
                                label="E-Mail-Adresse"
                                name="email"
                                type="email"
                                autoComplete="email"
                                required
                                value={formData.email}
                                onChange={handleChange}
                                error={errors.email}
                            />

                            <Input
                                label="Benutzername"
                                name="username"
                                type="text"
                                autoComplete="username"
                                required
                                value={formData.username}
                                onChange={handleChange}
                                error={errors.username}
                            />

                            <Input
                                label="Vollständiger Name"
                                name="name"
                                type="text"
                                autoComplete="name"
                                required
                                value={formData.name}
                                onChange={handleChange}
                                error={errors.name}
                            />

                            <Input
                                label="Passwort"
                                name="password"
                                type="password"
                                autoComplete="new-password"
                                required
                                value={formData.password}
                                onChange={handleChange}
                                error={errors.password}
                            />

                            <Input
                                label="Passwort bestätigen"
                                name="confirmPassword"
                                type="password"
                                autoComplete="new-password"
                                required
                                value={formData.confirmPassword}
                                onChange={handleChange}
                                error={errors.confirmPassword}
                            />
                        </div>

                        <Button
                            type="submit"
                            isLoading={isLoading}
                            loadingText="Konto wird erstellt..."
                        >
                            Konto erstellen
                        </Button>
                    </form>
                </CardContent>

                <CardFooter>
                    <p>
                        Bereits ein Konto? <Link href="/login">Jetzt anmelden</Link>
                    </p>
                </CardFooter>
            </Card>
        </div>
    );
}