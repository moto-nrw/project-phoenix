// Generic CRUD Service Factory

import { getSession } from "next-auth/react";
import type { EntityConfig, CrudService, PaginatedResponse } from "./types";

export function createCrudService<T>(config: EntityConfig<T>): CrudService<T> {
  const { api: apiConfig, service } = config;

  // Helper to get auth token
  const getToken = async () => {
    const session = await getSession();
    return session?.user?.token;
  };

  // Helper to make fetch requests with auth
  const fetchWithAuth = async (url: string, options: RequestInit = {}) => {
    const token = await getToken();
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      ...((options.headers as Record<string, string>) ?? {}),
    };

    if (token) {
      headers.Authorization = `Bearer ${token}`;
    }

    const response = await fetch(url, {
      ...options,
      headers,
      credentials: "include",
    });

    if (!response.ok) {
      const errorText = await response.text();
      console.error(`API error: ${response.status}`, errorText);
      throw new Error(`API error: ${response.status}`);
    }

    // Handle empty responses (204 No Content, or empty body from DELETE)
    const contentType = response.headers.get("content-type");
    const hasJson = contentType?.includes("application/json");
    const contentLength = response.headers.get("content-length");

    if (response.status === 204 || contentLength === "0" || !hasJson) {
      return null;
    }

    return response.json() as Promise<unknown>;
  };

  // Build endpoint URLs
  const endpoints = {
    list: apiConfig.endpoints?.list ?? apiConfig.basePath,
    get: apiConfig.endpoints?.get ?? `${apiConfig.basePath}/{id}`,
    create: apiConfig.endpoints?.create ?? apiConfig.basePath,
    update: apiConfig.endpoints?.update ?? `${apiConfig.basePath}/{id}`,
    delete: apiConfig.endpoints?.delete ?? `${apiConfig.basePath}/{id}`,
  };

  return {
    async getList(
      filters?: Record<string, unknown>,
    ): Promise<PaginatedResponse<T>> {
      try {
        // Build query string
        const params = new URLSearchParams();
        if (filters) {
          Object.entries(filters).forEach(([key, value]) => {
            if (value !== undefined && value !== null) {
              let stringValue: string;
              if (typeof value === "object") {
                stringValue = JSON.stringify(value);
              } else if (typeof value === "boolean") {
                stringValue = value.toString();
              } else if (typeof value === "number") {
                stringValue = value.toString();
              } else {
                stringValue = value as string;
              }
              params.append(key, stringValue);
            }
          });
        }

        const url = `${endpoints.list}${params.toString() ? `?${params.toString()}` : ""}`;
        const response = await fetchWithAuth(url);

        // Handle different response structures

        // Handle API wrapper with success/message/data structure
        if (
          response &&
          typeof response === "object" &&
          "success" in response &&
          "data" in response
        ) {
          const innerData = (response as { success: boolean; data: unknown })
            .data;

          // Check if inner data is a paginated response
          if (
            innerData &&
            typeof innerData === "object" &&
            "data" in innerData &&
            "pagination" in innerData
          ) {
            const paginatedData = innerData as PaginatedResponse<unknown>;
            // Handle response mapping for paginated data
            if (service?.mapResponse && Array.isArray(paginatedData.data)) {
              return {
                ...paginatedData,
                data: paginatedData.data.map((item: unknown) =>
                  service.mapResponse!(item),
                ),
              } as PaginatedResponse<T>;
            }
            return paginatedData as PaginatedResponse<T>;
          }

          // If inner data is an array
          if (Array.isArray(innerData)) {
            const mappedData: T[] = service?.mapResponse
              ? (innerData as unknown[]).map((item: unknown) =>
                  service.mapResponse!(item),
                )
              : (innerData as T[]);
            return {
              data: mappedData,
              pagination: {
                current_page: 1,
                page_size: mappedData.length,
                total_pages: 1,
                total_records: mappedData.length,
              },
            } as PaginatedResponse<T>;
          }

          // If inner data is an object with data array
          if (
            innerData &&
            typeof innerData === "object" &&
            "data" in innerData &&
            Array.isArray((innerData as { data: unknown[] }).data)
          ) {
            const dataArray = (
              innerData as {
                data: unknown[];
                pagination?: PaginatedResponse<T>["pagination"];
              }
            ).data;
            const mappedData: T[] = service?.mapResponse
              ? dataArray.map((item: unknown) => service.mapResponse!(item))
              : (dataArray as T[]);
            return {
              data: mappedData,
              pagination: (
                innerData as { pagination?: PaginatedResponse<T>["pagination"] }
              ).pagination ?? {
                current_page: 1,
                page_size: mappedData.length,
                total_pages: 1,
                total_records: mappedData.length,
              },
            } as PaginatedResponse<T>;
          }
        }

        // Check if it's already a paginated response (without wrapper)
        if (
          response &&
          typeof response === "object" &&
          "data" in response &&
          "pagination" in response
        ) {
          const paginatedResponse = response as PaginatedResponse<unknown>;
          // Handle response mapping for paginated data
          if (service?.mapResponse && Array.isArray(paginatedResponse.data)) {
            return {
              ...paginatedResponse,
              data: paginatedResponse.data.map((item: unknown) =>
                service.mapResponse!(item),
              ),
            } as PaginatedResponse<T>;
          }
          return paginatedResponse as PaginatedResponse<T>;
        }

        // If it's a direct array response (backward compatibility)
        if (Array.isArray(response)) {
          const mappedData: T[] = service?.mapResponse
            ? (response as unknown[]).map((item: unknown) =>
                service.mapResponse!(item),
              )
            : (response as T[]);
          return {
            data: mappedData,
            pagination: {
              current_page: 1,
              page_size: mappedData.length,
              total_pages: 1,
              total_records: mappedData.length,
            },
          };
        }

        // Handle wrapped response (e.g., { data: [...] })
        if (
          response &&
          typeof response === "object" &&
          "data" in response &&
          Array.isArray((response as { data: unknown[] }).data)
        ) {
          const dataArray = (
            response as {
              data: unknown[];
              pagination?: PaginatedResponse<T>["pagination"];
            }
          ).data;
          const mappedData: T[] = service?.mapResponse
            ? dataArray.map((item: unknown) => service.mapResponse!(item))
            : (dataArray as T[]);
          return {
            data: mappedData,
            pagination: (
              response as { pagination?: PaginatedResponse<T>["pagination"] }
            ).pagination ?? {
              current_page: 1,
              page_size: mappedData.length,
              total_pages: 1,
              total_records: mappedData.length,
            },
          } as PaginatedResponse<T>;
        }

        // Fallback - return empty paginated response
        console.warn("Unexpected response structure:", response);
        return {
          data: [],
          pagination: {
            current_page: 1,
            page_size: 50,
            total_pages: 0,
            total_records: 0,
          },
        };
      } catch (error) {
        console.error(`Error fetching ${config.name.plural}:`, error);
        throw error;
      }
    },

    async getOne(id: string): Promise<T> {
      try {
        // Check if there's a custom getOne method
        if (service?.customMethods?.getOne) {
          const result = await service.customMethods.getOne(id);
          return result as T;
        }

        const url = endpoints.get.replace("{id}", id);
        const response = await fetchWithAuth(url);

        const data = (response as { data?: unknown })?.data ?? response;
        return service?.mapResponse ? service.mapResponse(data) : (data as T);
      } catch (error) {
        console.error(`Error fetching ${config.name.singular} ${id}:`, error);
        throw error;
      }
    },

    async create(data: Partial<T>): Promise<T> {
      try {
        // Check if there's a custom create method
        if (service?.create) {
          const token = await getToken();
          const result = await service.create(data, token);

          // Apply after hook
          if (config.hooks?.afterCreate) {
            await config.hooks.afterCreate(result);
          }

          return result;
        }

        // Apply hooks
        if (config.hooks?.beforeCreate) {
          data = await config.hooks.beforeCreate(data);
        }

        const requestData = service?.mapRequest
          ? service.mapRequest(data)
          : data;

        const response = await fetchWithAuth(endpoints.create, {
          method: "POST",
          body: JSON.stringify(requestData),
        });

        const responseData = (response as { data?: unknown })?.data ?? response;
        const result = service?.mapResponse
          ? service.mapResponse(responseData)
          : (responseData as T);

        // Apply after hook
        if (config.hooks?.afterCreate) {
          await config.hooks.afterCreate(result);
        }

        return result;
      } catch (error) {
        console.error(`Error creating ${config.name.singular}:`, error);
        throw error;
      }
    },

    async update(id: string, data: Partial<T>): Promise<T> {
      try {
        // Check if there's a custom update method
        if (service?.update) {
          const token = await getToken();
          const result = await service.update(id, data, token);

          // Apply after hook
          if (config.hooks?.afterUpdate) {
            await config.hooks.afterUpdate(result);
          }

          return result;
        }

        // Apply hooks
        if (config.hooks?.beforeUpdate) {
          data = await config.hooks.beforeUpdate(id, data);
        }

        const url = endpoints.update.replace("{id}", id);
        const requestData = service?.mapRequest
          ? service.mapRequest(data)
          : data;

        const response = await fetchWithAuth(url, {
          method: "PUT",
          body: JSON.stringify(requestData),
        });

        const responseData = (response as { data?: unknown })?.data ?? response;
        const result = service?.mapResponse
          ? service.mapResponse(responseData)
          : (responseData as T);

        // Apply after hook
        if (config.hooks?.afterUpdate) {
          await config.hooks.afterUpdate(result);
        }

        return result;
      } catch (error) {
        console.error(`Error updating ${config.name.singular} ${id}:`, error);
        throw error;
      }
    },

    async delete(id: string): Promise<void> {
      try {
        // Apply hook
        if (config.hooks?.beforeDelete) {
          const shouldDelete = await config.hooks.beforeDelete(id);
          if (!shouldDelete) {
            throw new Error("Delete operation cancelled");
          }
        }

        const url = endpoints.delete.replace("{id}", id);

        await fetchWithAuth(url, {
          method: "DELETE",
        });

        // Apply after hook
        if (config.hooks?.afterDelete) {
          await config.hooks.afterDelete(id);
        }
      } catch (error) {
        console.error(`Error deleting ${config.name.singular} ${id}:`, error);
        throw error;
      }
    },
  };
}

// Export helper to create services with custom methods
export function createExtendedService<T>(
  config: EntityConfig<T>,
): CrudService<T> {
  const baseService = createCrudService(config);

  // Add custom methods if defined
  if (config.service?.customMethods) {
    return {
      ...baseService,
      ...config.service.customMethods,
    } as CrudService<T>;
  }

  return baseService;
}
