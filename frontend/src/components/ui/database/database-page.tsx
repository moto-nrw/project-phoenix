"use client";

import { useState, useEffect, useCallback, useMemo } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { 
  DatabaseListPage, 
  SelectFilter, 
  CreateFormModal, 
  DetailFormModal, 
} from "@/components/ui";
import { DatabaseForm } from "./database-form";
import { DatabaseDetailView } from "./database-detail-view";
import { DatabaseListItem } from "../database-list-item";
import { SimpleAlert } from "@/components/simple/SimpleAlert";
import { getDbOperationMessage } from "@/lib/use-notification";
import { createCrudService } from "@/lib/database/service-factory";
import type { EntityConfig } from "@/lib/database/types";

interface DatabasePageProps<T> {
  config: EntityConfig<T>;
  customListItem?: React.ComponentType<{ item: T; onClick: (item: T) => void }>;
}

export function DatabasePage<T extends { id: string }>({ 
  config,
  customListItem: CustomListItem
}: DatabasePageProps<T>) {
  const [showSuccessAlert, setShowSuccessAlert] = useState(false);
  const [successMessage, setSuccessMessage] = useState("");
  const [items, setItems] = useState<T[]>([]);
  const [allItems, setAllItems] = useState<T[]>([]); // For frontend search
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchFilter, setSearchFilter] = useState("");
  const [filters, setFilters] = useState<Record<string, string | null>>({});
  const [currentPage, setCurrentPage] = useState(1);
  const [pagination, setPagination] = useState<{
    current_page: number;
    page_size: number;
    total_pages: number;
    total_records: number;
  } | null>(null);
  
  // Determine search strategy (default to frontend)
  const searchStrategy = config.list.searchStrategy ?? 'frontend';
  const searchableFields = useMemo(() => config.list.searchableFields ?? ['name', 'title'], [config.list.searchableFields]);
  const minSearchLength = config.list.minSearchLength ?? 0;
  
  // Create Modal states
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createLoading, setCreateLoading] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  // Detail Modal states
  const [showDetailModal, setShowDetailModal] = useState(false);
  const [selectedItem, setSelectedItem] = useState<T | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);
  const [isEditing, setIsEditing] = useState(false);
  
  // State for async filter options
  const [asyncFilterOptions, setAsyncFilterOptions] = useState<Record<string, { value: string; label: string }[]>>({});

  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  // Create service instance - memoized to prevent recreation on every render
  const service = useMemo(() => createCrudService(config), [config]);

  // Function to fetch items with optional filters
  const fetchItems = useCallback(async (search?: string, customFilters?: Record<string, string | null>, page = 1) => {
    try {
      setLoading(true);

      // Prepare filters for API call
      const apiFilters: Record<string, string | number> = {
        ...Object.fromEntries(
          Object.entries(customFilters ?? filters).filter(([_, v]) => v !== null)
        ),
        page: page,
        pageSize: 50
      };
      
      // Only include search in API call for backend strategy
      if (searchStrategy === 'backend' && search && search.length >= minSearchLength) {
        apiFilters.search = search;
      }

      try {
        const data = await service.getList(apiFilters);
        
        // Ensure items is always an array
        const itemsArray = Array.isArray(data.data) ? data.data : [];
        
        if (searchStrategy === 'frontend') {
          // For frontend search, store all items and filter locally
          setAllItems(itemsArray);
          setItems(itemsArray); // Initially show all items
        } else {
          // For backend search, just set the returned items
          setItems(itemsArray);
        }
        
        setPagination(data.pagination ?? null);
        setError(null);
      } catch (apiErr) {
        console.error(`API error when fetching ${config.name.plural}:`, apiErr);
        setError(
          `Fehler beim Laden der ${config.name.plural}. Bitte versuchen Sie es später erneut.`
        );
        setItems([]);
        setAllItems([]);
        setPagination(null);
      }
    } catch (err) {
      console.error(`Error fetching ${config.name.plural}:`, err);
      setError(
        `Fehler beim Laden der ${config.name.plural}. Bitte versuchen Sie es später erneut.`
      );
      setItems([]);
      setAllItems([]);
      setPagination(null);
    } finally {
      setLoading(false);
    }
  }, [service, filters, searchStrategy, minSearchLength, config.name.plural]);

  // Helper function for frontend search and filtering
  const performFrontendSearch = useCallback((searchTerm: string, itemsToSearch: T[], activeFilters: Record<string, string | null>) => {
    let filteredItems = itemsToSearch;
    
    // Apply filters first
    Object.entries(activeFilters).forEach(([filterId, filterValue]) => {
      if (filterValue) {
        filteredItems = filteredItems.filter(item => {
          // Map filter IDs to actual field names
          let fieldName = filterId;
          if (filterId === 'groupId') {
            fieldName = 'group_id';
          }
          
          const itemValue = (item as Record<string, unknown>)[fieldName];
          
          // Special handling for boolean fields
          if (filterId === 'bus') {
            const boolValue = filterValue === 'true';
            return itemValue === boolValue;
          }
          
          // Special handling for supervisor_id filter
          if (filterId === 'supervisor_id') {
            const supervisors = (item as Record<string, unknown>).supervisors;
            if (Array.isArray(supervisors)) {
              return supervisors.some((sup: { staff_id?: unknown; id?: unknown }) => String(sup.staff_id) === filterValue || String(sup.id) === filterValue);
            }
            return false;
          }
          
          // Compare as strings for other fields
          return String(itemValue) === filterValue;
        });
      }
    });
    
    // Then apply search
    if (!searchTerm || searchTerm.length < minSearchLength) {
      return filteredItems;
    }
    
    const lowercaseSearch = searchTerm.toLowerCase();
    return filteredItems.filter(item => {
      // Search in all specified fields
      return searchableFields.some(field => {
        const value = (item as Record<string, unknown>)[field];
        if (typeof value === 'string') {
          return value.toLowerCase().includes(lowercaseSearch);
        }
        return false;
      });
    });
  }, [minSearchLength, searchableFields]);

  // Initial data load
  useEffect(() => {
    void fetchItems(undefined, undefined, currentPage);
  }, [currentPage, fetchItems]);
  
  // Load async filter options
  useEffect(() => {
    const loadAsyncFilters = async () => {
      if (!config.list.filters) return;
      
      for (const filter of config.list.filters) {
        if (typeof filter.options === 'function') {
          try {
            const options = await filter.options();
            setAsyncFilterOptions(prev => ({
              ...prev,
              [filter.id]: options
            }));
          } catch (error) {
            console.error(`Failed to load options for filter ${filter.id}:`, error);
          }
        } else if (filter.loadOptions) {
          try {
            const options = await filter.loadOptions();
            setAsyncFilterOptions(prev => ({
              ...prev,
              [filter.id]: options
            }));
          } catch (error) {
            console.error(`Failed to load options for filter ${filter.id}:`, error);
          }
        }
      }
    };
    
    void loadAsyncFilters();
  }, [config.list.filters]);

  // Handle search and filter changes
  useEffect(() => {
    if (searchStrategy === 'frontend') {
      // For frontend search, filter locally without API call
      const filteredItems = performFrontendSearch(searchFilter, allItems, filters);
      setItems(filteredItems);
    } else {
      // For backend search, debounce and call API
      setCurrentPage(1);
      const timer = setTimeout(() => {
        void fetchItems(searchFilter, filters, 1);
      }, 300);

      return () => clearTimeout(timer);
    }
  }, [searchFilter, filters, searchStrategy, allItems, fetchItems, performFrontendSearch]);

  if (status === "loading") {
    return <div />; // Let DatabaseListPage handle the loading state
  }

  const handleSelectItem = async (item: T) => {
    setSelectedItem(item);
    setShowDetailModal(true);
    setDetailError(null);
    
    // Fetch fresh data for the selected item
    try {
      setDetailLoading(true);
      const freshData = await service.getOne(item.id);
      setSelectedItem(freshData);
    } catch (err) {
      console.error(`Error fetching ${config.name.singular} details:`, err);
      setDetailError(`Fehler beim Laden der ${config.name.singular}-Daten.`);
    } finally {
      setDetailLoading(false);
    }
  };

  // Handle item creation
  const handleCreateItem = async (itemData: Partial<T>) => {
    try {
      setCreateLoading(true);
      setCreateError(null);

      // Transform data if needed
      if (config.form.transformBeforeSubmit) {
        itemData = config.form.transformBeforeSubmit(itemData);
      }

      // Create the item
      const newItem = await service.create(itemData);
      
      // Call onCreateSuccess callback if defined
      if (config.onCreateSuccess) {
        config.onCreateSuccess(newItem);
      }
      
      // Show success notification
      const displayName = config.list.item.title(newItem);
      setSuccessMessage(getDbOperationMessage('create', config.name.singular, displayName));
      setShowSuccessAlert(true);
      
      // Close modal and refresh list
      setShowCreateModal(false);
      await fetchItems(searchFilter, filters, currentPage);
      
      // For devices with API keys, automatically open the detail modal
      if ('api_key' in newItem && newItem.api_key) {
        setSelectedItem(newItem);
        setShowDetailModal(true);
      }
    } catch (err) {
      console.error(`Error creating ${config.name.singular}:`, err);
      setCreateError(
        `Fehler beim Erstellen des ${config.name.singular}. Bitte versuchen Sie es später erneut.`
      );
      throw err; // Re-throw for form to handle
    } finally {
      setCreateLoading(false);
    }
  };

  // Handle item update
  const handleUpdateItem = async (itemData: Partial<T>) => {
    if (!selectedItem) return;
    
    try {
      setDetailLoading(true);
      setDetailError(null);

      // Transform data if needed
      if (config.form.transformBeforeSubmit) {
        itemData = config.form.transformBeforeSubmit(itemData);
      }

      // Update item
      await service.update(selectedItem.id, itemData);
      
      const displayName = config.list.item.title(selectedItem);
      setSuccessMessage(getDbOperationMessage('update', config.name.singular, displayName));
      setShowSuccessAlert(true);
      
      // Refresh the selected item data
      const refreshedItem = await service.getOne(selectedItem.id);
      setSelectedItem(refreshedItem);
      setIsEditing(false);
      
      // Refresh the list
      await fetchItems(searchFilter, filters, currentPage);
    } catch (err) {
      console.error(`Error updating ${config.name.singular}:`, err);
      setDetailError(
        `Fehler beim Aktualisieren des ${config.name.singular}. Bitte versuchen Sie es später erneut.`
      );
      throw err;
    } finally {
      setDetailLoading(false);
    }
  };

  // Handle item deletion
  const handleDeleteItem = async () => {
    if (!selectedItem) return;
    
    const confirmMessage = config.labels?.deleteConfirmation ?? 
      `Sind Sie sicher, dass Sie diesen ${config.name.singular} löschen möchten?`;
    
    if (window.confirm(confirmMessage)) {
      try {
        setDetailLoading(true);
        await service.delete(selectedItem.id);
        
        const displayName = config.list.item.title(selectedItem);
        setSuccessMessage(getDbOperationMessage('delete', config.name.singular, displayName));
        setShowSuccessAlert(true);
        
        // Close modal and refresh list
        setShowDetailModal(false);
        setSelectedItem(null);
        await fetchItems(searchFilter, filters, currentPage);
      } catch (err) {
        console.error(`Error deleting ${config.name.singular}:`, err);
        setDetailError(
          `Fehler beim Löschen des ${config.name.singular}. Bitte versuchen Sie es später erneut.`
        );
      } finally {
        setDetailLoading(false);
      }
    }
  };

  // Process dynamic filters
  const processedFilters = config.list.filters?.map(filter => {
    // Check for async options first
    if (asyncFilterOptions[filter.id]) {
      return {
        ...filter,
        options: asyncFilterOptions[filter.id]
      };
    }
    
    if (filter.options === 'dynamic') {
      // Special handling for groupId filter to use group_name as label
      if (filter.id === 'groupId') {
        const groupMap = new Map<string, string>();
        
        allItems.forEach(item => {
          const groupId = (item as Record<string, unknown>).group_id;
          const groupName = (item as Record<string, unknown>).group_name;
          if (groupId && groupName && 
              (typeof groupId === 'string' || typeof groupId === 'number') && 
              (typeof groupName === 'string' || typeof groupName === 'number')) {
            groupMap.set(String(groupId), String(groupName));
          }
        });
        
        return {
          ...filter,
          options: Array.from(groupMap.entries())
            .sort((a, b) => a[1].localeCompare(b[1]))
            .map(([value, label]) => ({ value, label }))
        };
      }
      
      // Special handling for ag_category_id filter to use category_name as label
      if (filter.id === 'ag_category_id') {
        const categoryMap = new Map<string, string>();
        
        allItems.forEach(item => {
          const categoryId = (item as Record<string, unknown>).ag_category_id;
          const categoryName = (item as Record<string, unknown>).category_name;
          if (categoryId && categoryName && 
              (typeof categoryId === 'string' || typeof categoryId === 'number') && 
              (typeof categoryName === 'string' || typeof categoryName === 'number')) {
            categoryMap.set(String(categoryId), String(categoryName));
          }
        });
        
        return {
          ...filter,
          options: Array.from(categoryMap.entries())
            .sort((a, b) => a[1].localeCompare(b[1]))
            .map(([value, label]) => ({ value, label }))
        };
      }
      
      // Default behavior for other dynamic filters
      const uniqueValues = Array.from(
        new Set(
          allItems
            .filter(item => (item as Record<string, unknown>)[filter.id])
            .map(item => String((item as Record<string, unknown>)[filter.id]))
        )
      ).sort();
      
      return {
        ...filter,
        options: uniqueValues.map(value => ({
          value,
          label: value
        }))
      };
    }
    return filter;
  });

  // Render filters
  const renderFilters = () => {
    if (!processedFilters || processedFilters.length === 0) {
      return null;
    }

    return (
      <div className="flex flex-wrap gap-4">
        {processedFilters.map(filter => {
          if (filter.type === 'select') {
            return (
              <div key={filter.id} className="md:max-w-xs">
                <SelectFilter
                  id={filter.id}
                  label={filter.label}
                  value={filters[filter.id] ?? null}
                  onChange={(value) => setFilters(prev => ({ ...prev, [filter.id]: value }))}
                  options={Array.isArray(filter.options) ? filter.options : []}
                  placeholder={`Alle ${filter.label}`}
                />
              </div>
            );
          }
          // Add more filter types as needed
          return null;
        })}
      </div>
    );
  };

  return (
    <>
      <DatabaseListPage
        userName={session?.user?.name ?? "Benutzer"}
        title={`${config.name.singular} auswählen`}
        description={config.list.description}
        backUrl={config.backUrl}
        listTitle={`${config.name.singular}liste`}
        searchPlaceholder={config.list.searchPlaceholder}
        searchValue={searchFilter}
        onSearchChange={setSearchFilter}
        filters={renderFilters()}
        addButton={config.list.features?.create !== false ? {
          label: config.labels?.createButton ?? `Neuen ${config.name.singular} erstellen`,
          onClick: () => setShowCreateModal(true)
        } : undefined}
        infoSection={config.list.infoSection}
        items={items}
        loading={loading}
        error={error}
        onRetry={() => fetchItems(searchFilter, filters, currentPage)}
        itemLabel={{ singular: config.name.singular, plural: config.name.plural }}
        renderItem={(item: T) => {
          if (CustomListItem) {
            return <CustomListItem item={item} onClick={handleSelectItem} />;
          }
          
          return (
            <DatabaseListItem
              id={item.id}
              title={config.list.item.title(item)}
              subtitle={config.list.item.subtitle?.(item) ?? config.list.item.description?.(item)}
              onClick={() => handleSelectItem(item)}
              leftIcon={config.list.item.avatar ? (
                <div
                  className={`h-10 w-10 rounded-full bg-gradient-to-br ${config.theme.avatarGradient} flex items-center justify-center text-white font-medium`}
                >
                  {config.list.item.avatar.text(item)}
                </div>
              ) : undefined}
              badges={config.list.item.badges?.filter(badge => 
                !badge.showWhen || badge.showWhen(item)
              ).map((badge, index) => (
                <span key={index} className={`inline-flex items-center px-2 py-1 rounded-full text-xs font-medium ${badge.color}`}>
                  {typeof badge.label === 'function' ? badge.label(item) : badge.label}
                </span>
              ))}
            />
          );
        }}
        pagination={pagination}
        onPageChange={setCurrentPage}
      />

      {/* Create Modal */}
      <CreateFormModal
        isOpen={showCreateModal}
        onClose={() => {
          setShowCreateModal(false);
          setCreateError(null);
        }}
        title={config.labels?.createModalTitle ?? `Neuer ${config.name.singular}`}
        size="lg"
      >
        {createError && (
          <div className="mb-4 rounded-lg bg-red-50 p-4 text-red-800">
            <p>{createError}</p>
          </div>
        )}
        
        <DatabaseForm
          sections={config.form.sections.map(section => ({
            title: section.title,
            subtitle: section.subtitle,
            fields: section.fields.map(field => ({
              name: field.name,
              label: field.label,
              type: field.type,
              required: field.required,
              placeholder: field.placeholder,
              options: field.options,
              validation: field.validation,
              component: field.component,
              helperText: field.helperText,
              autoComplete: field.autoComplete,
              colSpan: field.colSpan,
              min: field.min,
              max: field.max,
            })),
            columns: section.columns,
            backgroundColor: section.backgroundColor,
          }))}
          initialData={config.form.defaultValues}
          onSubmit={handleCreateItem}
          onCancel={() => setShowCreateModal(false)}
          isLoading={createLoading}
          theme={config.theme}
          submitLabel="Erstellen"
        />
      </CreateFormModal>

      {/* Detail/Edit Modal */}
      <DetailFormModal
        isOpen={showDetailModal}
        onClose={() => {
          setShowDetailModal(false);
          setSelectedItem(null);
          setIsEditing(false);
          setDetailError(null);
        }}
        title={isEditing 
          ? (config.labels?.editModalTitle ?? `${config.name.singular} bearbeiten`)
          : (config.labels?.detailModalTitle ?? `${config.name.singular}details`)
        }
        size="xl"
        loading={detailLoading}
        error={detailError}
        onRetry={() => selectedItem && handleSelectItem(selectedItem)}
      >
        {selectedItem && !isEditing && (
          <DatabaseDetailView
            theme={config.theme}
            header={config.detail.header ? {
              title: config.detail.header.title(selectedItem),
              subtitle: config.detail.header.subtitle?.(selectedItem),
              avatar: config.detail.header.avatar ? {
                text: config.detail.header.avatar.text(selectedItem),
                size: config.detail.header.avatar.size
              } : undefined,
              badges: config.detail.header.badges?.filter(badge => 
                badge.showWhen(selectedItem)
              ).map(badge => ({
                label: typeof badge.label === 'function' ? badge.label(selectedItem) : badge.label,
                color: badge.color
              }))
            } : undefined}
            sections={config.detail.sections.map(section => ({
              ...section,
              items: section.items.map(item => ({
                label: item.label,
                value: item.value(selectedItem)
              }))
            }))}
            actions={{
              onEdit: config.detail.actions?.edit !== false ? () => setIsEditing(true) : undefined,
              onDelete: config.detail.actions?.delete !== false ? () => void handleDeleteItem() : undefined,
              custom: config.detail.actions?.custom?.map(action => ({
                ...action,
                onClick: () => action.onClick(selectedItem)
              }))
            }}
          />
        )}
        
        {selectedItem && isEditing && (
          <DatabaseForm
            sections={config.form.sections.filter(section => 
              // Filter out password section when editing teachers
              !(section.title === 'Zugangsdaten' && config.name.singular === 'Pädagogische Fachkraft')
            ).map(section => ({
              title: section.title,
              subtitle: section.subtitle,
              fields: section.fields.map(field => ({
                name: field.name,
                label: field.label,
                type: field.type,
                required: field.required,
                placeholder: field.placeholder,
                options: field.options,
                validation: field.validation,
                component: field.component,
                helperText: field.helperText,
                autoComplete: field.autoComplete,
                colSpan: field.colSpan,
                min: field.min,
                max: field.max,
              })),
              columns: section.columns,
              backgroundColor: section.backgroundColor,
            }))}
            initialData={selectedItem}
            onSubmit={handleUpdateItem}
            onCancel={() => setIsEditing(false)}
            isLoading={detailLoading}
            theme={config.theme}
            submitLabel="Speichern"
          />
        )}
      </DetailFormModal>
      
      {/* Success Alert */}
      {showSuccessAlert && (
        <SimpleAlert
          type="success"
          message={successMessage}
          autoClose
          duration={3000}
          onClose={() => setShowSuccessAlert(false)}
        />
      )}
    </>
  );
}