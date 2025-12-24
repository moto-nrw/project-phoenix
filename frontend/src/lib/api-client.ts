import api from "./api";
import axios from "axios";
import type { AxiosResponse, AxiosRequestConfig } from "axios";

/**
 * GET request wrapper
 */
export async function apiGet<T = unknown>(
  url: string,
  token?: string,
  config?: AxiosRequestConfig,
): Promise<AxiosResponse<T>> {
  const headers: Record<string, string> = {
    ...(config?.headers as Record<string, string>),
  };
  if (token) {
    headers.Authorization = `Bearer ${token}`;
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
 */
export async function apiPost<T = unknown>(
  url: string,
  data?: unknown,
  token?: string,
  config?: AxiosRequestConfig,
): Promise<AxiosResponse<T>> {
  const headers: Record<string, string> = {
    ...(config?.headers as Record<string, string>),
  };
  if (token) {
    headers.Authorization = `Bearer ${token}`;
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
 */
export async function apiPut<T = unknown>(
  url: string,
  data?: unknown,
  token?: string,
  config?: AxiosRequestConfig,
): Promise<AxiosResponse<T>> {
  const headers: Record<string, string> = {
    ...(config?.headers as Record<string, string>),
  };
  if (token) {
    headers.Authorization = `Bearer ${token}`;
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
 */
export async function apiDelete<T = unknown>(
  url: string,
  token?: string,
  config?: AxiosRequestConfig,
): Promise<AxiosResponse<T>> {
  const headers: Record<string, string> = {
    ...(config?.headers as Record<string, string>),
  };
  if (token) {
    headers.Authorization = `Bearer ${token}`;
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
