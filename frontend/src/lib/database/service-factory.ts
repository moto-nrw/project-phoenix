// Generic CRUD Service Factory

import { getSession } from 'next-auth/react';
import type { 
  EntityConfig, 
  CrudService, 
  PaginatedResponse 
} from './types';

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
    const headers: HeadersInit = {
      'Content-Type': 'application/json',
      ...(options.headers ?? {}),
    };
    
    if (token) {
      headers.Authorization = `Bearer ${token}`;
    }
    
    const response = await fetch(url, {
      ...options,
      headers,
      credentials: 'include',
    });
    
    if (!response.ok) {
      const errorText = await response.text();
      console.error(`API error: ${response.status}`, errorText);
      throw new Error(`API error: ${response.status}`);
    }
    
    // Handle empty responses (204 No Content, or empty body from DELETE)
    const contentType = response.headers.get('content-type');
    const hasJson = contentType?.includes('application/json');
    const contentLength = response.headers.get('content-length');
    
    if (response.status === 204 || contentLength === '0' || !hasJson) {
      return null;
    }
    
    return response.json();
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
    async getList(filters?: Record<string, unknown>): Promise<PaginatedResponse<T>> {
      try {
        // Build query string
        const params = new URLSearchParams();
        if (filters) {
          Object.entries(filters).forEach(([key, value]) => {
            if (value !== undefined && value !== null) {
              params.append(key, String(value));
            }
          });
        }
        
        const url = `${endpoints.list}${params.toString() ? `?${params.toString()}` : ''}`;
        const response = await fetchWithAuth(url);
        
        // Handle response mapping
        if (service?.mapResponse && response.data) {
          return {
            ...response,
            data: response.data.map(service.mapResponse),
          };
        }
        
        return response;
      } catch (error) {
        console.error(`Error fetching ${config.name.plural}:`, error);
        throw error;
      }
    },
    
    async getOne(id: string): Promise<T> {
      try {
        const url = endpoints.get.replace('{id}', id);
        const response = await fetchWithAuth(url);
        
        const data = response.data ?? response;
        return service?.mapResponse ? service.mapResponse(data) : data;
      } catch (error) {
        console.error(`Error fetching ${config.name.singular} ${id}:`, error);
        throw error;
      }
    },
    
    async create(data: Partial<T>): Promise<T> {
      try {
        // Apply hooks
        if (config.hooks?.beforeCreate) {
          data = await config.hooks.beforeCreate(data);
        }
        
        const requestData = service?.mapRequest ? service.mapRequest(data) : data;
        
        const response = await fetchWithAuth(endpoints.create, {
          method: 'POST',
          body: JSON.stringify(requestData),
        });
        
        const result = service?.mapResponse 
          ? service.mapResponse(response.data ?? response)
          : (response.data ?? response);
        
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
        // Apply hooks
        if (config.hooks?.beforeUpdate) {
          data = await config.hooks.beforeUpdate(id, data);
        }
        
        const url = endpoints.update.replace('{id}', id);
        const requestData = service?.mapRequest ? service.mapRequest(data) : data;
        
        const response = await fetchWithAuth(url, {
          method: 'PUT',
          body: JSON.stringify(requestData),
        });
        
        const result = service?.mapResponse 
          ? service.mapResponse(response.data ?? response)
          : (response.data ?? response);
        
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
            throw new Error('Delete operation cancelled');
          }
        }
        
        const url = endpoints.delete.replace('{id}', id);
        
        await fetchWithAuth(url, {
          method: 'DELETE',
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
  config: EntityConfig<T>
): CrudService<T> & Record<string, (...args: unknown[]) => unknown> {
  const baseService = createCrudService(config);
  
  // Add custom methods if defined
  if (config.service?.customMethods) {
    return {
      ...baseService,
      ...config.service.customMethods,
    };
  }
  
  return baseService;
}