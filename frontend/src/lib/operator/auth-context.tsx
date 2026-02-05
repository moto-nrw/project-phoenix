"use client";

import React, {
  createContext,
  useContext,
  useEffect,
  useState,
  useCallback,
  useMemo,
} from "react";
import { useRouter } from "next/navigation";

interface Operator {
  id: string;
  displayName: string;
  email: string;
}

interface OperatorAuthContextType {
  operator: Operator | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  updateOperator: (updates: Partial<Operator>) => void;
}

interface SessionResponse {
  id: string;
  displayName: string;
  email: string;
}

interface LoginResponse {
  success: boolean;
  operator: {
    id: string;
    displayName: string;
    email: string;
  };
}

interface ErrorResponse {
  error?: string;
}

const OperatorAuthContext = createContext<OperatorAuthContextType | undefined>(
  undefined,
);

export function OperatorAuthProvider({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  const router = useRouter();
  const [operator, setOperator] = useState<Operator | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  // Check authentication status on mount
  useEffect(() => {
    const checkAuth = async () => {
      try {
        const response = await fetch("/api/operator/me");
        if (response.ok) {
          const data = (await response.json()) as SessionResponse;
          setOperator(data);
        } else {
          setOperator(null);
        }
      } catch (error) {
        console.error("Failed to check operator auth:", error);
        setOperator(null);
      } finally {
        setIsLoading(false);
      }
    };

    void checkAuth();
  }, []);

  const login = useCallback(
    async (email: string, password: string) => {
      const response = await fetch("/api/operator/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password }),
      });

      if (!response.ok) {
        const errorData = (await response.json()) as ErrorResponse;
        throw new Error(errorData.error ?? "Login failed");
      }

      const data = (await response.json()) as LoginResponse;
      setOperator(data.operator);
      router.push("/operator/suggestions");
    },
    [router],
  );

  const updateOperator = useCallback((updates: Partial<Operator>) => {
    setOperator((prev) => (prev ? { ...prev, ...updates } : prev));
  }, []);

  const logout = useCallback(async () => {
    try {
      await fetch("/api/operator/logout", { method: "POST" });
    } catch (error) {
      console.error("Logout error:", error);
    } finally {
      setOperator(null);
      router.push("/operator/login");
    }
  }, [router]);

  const contextValue = useMemo(
    () => ({
      operator,
      isAuthenticated: operator !== null,
      isLoading,
      login,
      logout,
      updateOperator,
    }),
    [operator, isLoading, login, logout, updateOperator],
  );

  return (
    <OperatorAuthContext.Provider value={contextValue}>
      {children}
    </OperatorAuthContext.Provider>
  );
}

export function useOperatorAuth() {
  const context = useContext(OperatorAuthContext);
  if (context === undefined) {
    throw new Error(
      "useOperatorAuth must be used within an OperatorAuthProvider",
    );
  }
  return context;
}
