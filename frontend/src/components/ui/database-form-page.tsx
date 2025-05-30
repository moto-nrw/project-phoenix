"use client";

import { useState, useEffect } from "react";
import type { ReactNode } from "react";
import { useRouter } from "next/navigation";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { PageHeader, SectionTitle, ResponsiveLayout } from "@/components/dashboard";

// Configuration for the form page
export interface DatabaseFormPageConfig<TFormData = any, TLoadData = any> {
  // Basic configuration
  title: string;
  backUrl: string;
  resourceName: string;
  
  // Form configuration
  FormComponent: React.ComponentType<any>; // Allow any form component
  
  // Data handling
  onCreate: (data: TFormData) => Promise<any>;
  onDataLoad?: () => Promise<TLoadData>;
  mapFormData?: (data: TFormData, loadedData?: TLoadData) => any;
  
  // Navigation
  successRedirectUrl?: string | ((created: any) => string);
  
  // Authentication
  requiresAuth?: boolean;
  
  // UI customization
  successMessage?: string | ((created: any) => ReactNode);
  hints?: ReactNode;
  beforeForm?: ReactNode;
  afterForm?: ReactNode;
  
  // Form props
  formProps?: Record<string, any>;
  initialFormData?: any;
}

interface DatabaseFormPageProps<TFormData = any, TLoadData = any> {
  config: DatabaseFormPageConfig<TFormData, TLoadData>;
}

export function DatabaseFormPage<TFormData = any, TLoadData = any>({ 
  config 
}: DatabaseFormPageProps<TFormData, TLoadData>) {
  const router = useRouter();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loadedData, setLoadedData] = useState<TLoadData | null>(null);
  const [successData, setSuccessData] = useState<any>(null);
  
  // Handle authentication if required
  const { status } = useSession({
    required: config.requiresAuth ?? false,
    onUnauthenticated() {
      if (config.requiresAuth) {
        redirect("/");
      }
    },
  });

  // Load data on mount if onDataLoad is provided
  useEffect(() => {
    const loadData = async () => {
      if (!config.onDataLoad) return;
      
      try {
        setLoading(true);
        setError(null);
        const data = await config.onDataLoad();
        setLoadedData(data);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Fehler beim Laden der Daten");
        console.error("Error loading data:", err);
      } finally {
        setLoading(false);
      }
    };

    void loadData();
  }, [config.onDataLoad]);

  // Handle form submission
  const handleSubmit = async (formData: TFormData) => {
    try {
      setSaving(true);
      setError(null);
      
      // Map form data if mapper is provided
      const dataToSave = config.mapFormData 
        ? config.mapFormData(formData, loadedData || undefined)
        : formData;
      
      // Create the resource
      const created = await config.onCreate(dataToSave);
      setSuccessData(created);
      
      // Handle success message if provided
      if (config.successMessage) {
        const message = typeof config.successMessage === "function"
          ? config.successMessage(created)
          : config.successMessage;
        
        // If message is a ReactNode, we need to handle it differently
        if (message && typeof message !== "string") {
          // For now, we'll skip showing complex success messages
          // In a real implementation, we might show a modal or toast
        }
      }
      
      // Navigate to success URL
      const redirectUrl = config.successRedirectUrl
        ? (typeof config.successRedirectUrl === "function"
            ? config.successRedirectUrl(created)
            : config.successRedirectUrl)
        : config.backUrl;
      
      router.push(redirectUrl);
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : `Fehler beim Erstellen von ${config.resourceName}`;
      setError(errorMessage);
      throw err; // Re-throw for form to handle
    } finally {
      setSaving(false);
    }
  };

  // Handle cancel
  const handleCancel = () => {
    router.push(config.backUrl);
  };

  // Show loading state during auth check
  if (config.requiresAuth && status === "loading") {
    return <div />;
  }

  // Main render
  return (
    <ResponsiveLayout>
      <PageHeader title={config.title} backUrl={config.backUrl} />
      
      <main className="mx-auto max-w-4xl p-4 pb-24 lg:pb-8">
          {/* Error display */}
          {error && (
            <div className="mb-6 rounded-lg bg-red-50 p-4 text-red-800">
              <p>{error}</p>
            </div>
          )}
          
          {/* Loading state for initial data */}
          {loading && config.onDataLoad ? (
            <div className="flex justify-center py-12">
              <div className="text-center">
                <div className="mb-4 inline-block h-12 w-12 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
                <p className="text-gray-600">Daten werden geladen...</p>
              </div>
            </div>
          ) : (
            <>
              {/* Before form content */}
              {config.beforeForm}
              
              {/* Form component */}
              <config.FormComponent
                onSubmit={handleSubmit}
                onCancel={handleCancel}
                loading={saving}
                initialData={config.initialFormData}
                {...(config.formProps || {})}
                {...(loadedData ? { loadedData } : {})}
                // Support alternative prop names for compatibility
                onSubmitAction={handleSubmit}
                onCancelAction={handleCancel}
                isLoading={saving}
              />
              
              {/* After form content */}
              {config.afterForm}
              
              {/* Hints section */}
              {config.hints && (
                <div className="mt-8">
                  {typeof config.hints === "string" ? (
                    <>
                      <SectionTitle title="Hinweise" />
                      <div className="rounded-lg bg-blue-50 p-4">
                        <p className="text-sm text-blue-800">{config.hints}</p>
                      </div>
                    </>
                  ) : (
                    config.hints
                  )}
                </div>
              )}
            </>
          )}
        </main>
    </ResponsiveLayout>
  );
}

// Helper function to create a form page configuration
export function createFormPageConfig<TFormData = any, TLoadData = any>(
  config: DatabaseFormPageConfig<TFormData, TLoadData>
): DatabaseFormPageConfig<TFormData, TLoadData> {
  return config;
}