import api from "./api";
import type { AxiosResponse, AxiosRequestConfig } from "axios";

/**
 * GET request wrapper
 */
export async function apiGet<T = unknown>(url: string, token?: string, config?: AxiosRequestConfig): Promise<AxiosResponse<T>> {
  const headers: Record<string, string> = { ...(config?.headers as Record<string, string> ?? {}) };
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  return api.get<T>(url, { ...config, headers });
}

/**
 * POST request wrapper
 */
export async function apiPost<T = unknown>(url: string, data?: unknown, token?: string, config?: AxiosRequestConfig): Promise<AxiosResponse<T>> {
  const headers: Record<string, string> = { ...(config?.headers as Record<string, string> ?? {}) };
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  return api.post<T>(url, data, { ...config, headers });
}

/**
 * PUT request wrapper
 */
export async function apiPut<T = unknown>(url: string, data?: unknown, token?: string, config?: AxiosRequestConfig): Promise<AxiosResponse<T>> {
  const headers: Record<string, string> = { ...(config?.headers as Record<string, string> ?? {}) };
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  return api.put<T>(url, data, { ...config, headers });
}

/**
 * DELETE request wrapper
 */
export async function apiDelete<T = unknown>(url: string, token?: string, config?: AxiosRequestConfig): Promise<AxiosResponse<T>> {
  const headers: Record<string, string> = { ...(config?.headers as Record<string, string> ?? {}) };
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  return api.delete<T>(url, { ...config, headers });
}