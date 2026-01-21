import api from "./api";
import axios from "axios";
import type { AxiosResponse, AxiosRequestConfig } from "axios";

/**
 * GET request wrapper
 * BetterAuth: Uses Cookie header instead of Bearer token
 */
export async function apiGet<T = unknown>(
  url: string,
  cookieHeader?: string,
  config?: AxiosRequestConfig,
): Promise<AxiosResponse<T>> {
  const headers: Record<string, string> = {
    ...(config?.headers as Record<string, string>),
  };
  if (cookieHeader) {
    headers.Cookie = cookieHeader;
  }

  try {
    return await api.get<T>(url, { ...config, headers });
  } catch (error) {
    // Re-throw the error to let the route handler deal with it
    if (axios.isAxiosError(error) && error.response?.status === 401) {
      // Throw a specific error for 401s so route handlers can identify it
      throw new Error(`API error (401): Unauthorized`);
    }
    throw error;
  }
}

/**
 * POST request wrapper
 * BetterAuth: Uses Cookie header instead of Bearer token
 */
export async function apiPost<T = unknown>(
  url: string,
  data?: unknown,
  cookieHeader?: string,
  config?: AxiosRequestConfig,
): Promise<AxiosResponse<T>> {
  const headers: Record<string, string> = {
    ...(config?.headers as Record<string, string>),
  };
  if (cookieHeader) {
    headers.Cookie = cookieHeader;
  }

  try {
    return await api.post<T>(url, data, { ...config, headers });
  } catch (error) {
    // Re-throw the error to let the route handler deal with it
    if (axios.isAxiosError(error) && error.response?.status === 401) {
      // Throw a specific error for 401s so route handlers can identify it
      throw new Error(`API error (401): Unauthorized`);
    }
    throw error;
  }
}

/**
 * PUT request wrapper
 * BetterAuth: Uses Cookie header instead of Bearer token
 */
export async function apiPut<T = unknown>(
  url: string,
  data?: unknown,
  cookieHeader?: string,
  config?: AxiosRequestConfig,
): Promise<AxiosResponse<T>> {
  const headers: Record<string, string> = {
    ...(config?.headers as Record<string, string>),
  };
  if (cookieHeader) {
    headers.Cookie = cookieHeader;
  }

  try {
    return await api.put<T>(url, data, { ...config, headers });
  } catch (error) {
    // Re-throw the error to let the route handler deal with it
    if (axios.isAxiosError(error) && error.response?.status === 401) {
      // Throw a specific error for 401s so route handlers can identify it
      throw new Error(`API error (401): Unauthorized`);
    }
    throw error;
  }
}

/**
 * DELETE request wrapper
 * BetterAuth: Uses Cookie header instead of Bearer token
 */
export async function apiDelete<T = unknown>(
  url: string,
  cookieHeader?: string,
  config?: AxiosRequestConfig,
): Promise<AxiosResponse<T>> {
  const headers: Record<string, string> = {
    ...(config?.headers as Record<string, string>),
  };
  if (cookieHeader) {
    headers.Cookie = cookieHeader;
  }

  try {
    return await api.delete<T>(url, { ...config, headers });
  } catch (error) {
    // Re-throw the error to let the route handler deal with it
    if (axios.isAxiosError(error) && error.response?.status === 401) {
      // Throw a specific error for 401s so route handlers can identify it
      throw new Error(`API error (401): Unauthorized`);
    }
    throw error;
  }
}
