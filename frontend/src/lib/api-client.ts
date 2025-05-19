import api from "./api";
import type { AxiosResponse } from "axios";

/**
 * GET request wrapper
 */
export async function apiGet<T = any>(url: string, token?: string, config?: any): Promise<AxiosResponse<T>> {
  const headers = config?.headers || {};
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  return api.get<T>(url, { ...config, headers });
}

/**
 * POST request wrapper
 */
export async function apiPost<T = any>(url: string, data?: any, token?: string, config?: any): Promise<AxiosResponse<T>> {
  const headers = config?.headers || {};
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  return api.post<T>(url, data, { ...config, headers });
}

/**
 * PUT request wrapper
 */
export async function apiPut<T = any>(url: string, data?: any, token?: string, config?: any): Promise<AxiosResponse<T>> {
  const headers = config?.headers || {};
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  return api.put<T>(url, data, { ...config, headers });
}

/**
 * DELETE request wrapper
 */
export async function apiDelete<T = any>(url: string, token?: string, config?: any): Promise<AxiosResponse<T>> {
  const headers = config?.headers || {};
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  return api.delete<T>(url, { ...config, headers });
}