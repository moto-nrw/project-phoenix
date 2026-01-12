import { describe, it, expect, vi } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useDeleteConfirmation } from "./useDeleteConfirmation";

describe("useDeleteConfirmation", () => {
  it("should initialize with showConfirmModal as false", () => {
    const setShowDetailModal = vi.fn();
    const { result } = renderHook(() =>
      useDeleteConfirmation(setShowDetailModal),
    );

    expect(result.current.showConfirmModal).toBe(false);
  });

  it("should have stable function references", () => {
    const setShowDetailModal = vi.fn();
    const { result, rerender } = renderHook(() =>
      useDeleteConfirmation(setShowDetailModal),
    );

    const initialHandleDeleteClick = result.current.handleDeleteClick;
    const initialHandleDeleteCancel = result.current.handleDeleteCancel;
    const initialConfirmDelete = result.current.confirmDelete;

    rerender();

    expect(result.current.handleDeleteClick).toBe(initialHandleDeleteClick);
    expect(result.current.handleDeleteCancel).toBe(initialHandleDeleteCancel);
    expect(result.current.confirmDelete).toBe(initialConfirmDelete);
  });

  describe("handleDeleteClick", () => {
    it("should close detail modal and open confirm modal", () => {
      const setShowDetailModal = vi.fn();
      const { result } = renderHook(() =>
        useDeleteConfirmation(setShowDetailModal),
      );

      act(() => {
        result.current.handleDeleteClick();
      });

      expect(setShowDetailModal).toHaveBeenCalledWith(false);
      expect(result.current.showConfirmModal).toBe(true);
    });
  });

  describe("handleDeleteCancel", () => {
    it("should close confirm modal and reopen detail modal", () => {
      const setShowDetailModal = vi.fn();
      const { result } = renderHook(() =>
        useDeleteConfirmation(setShowDetailModal),
      );

      // First open the confirm modal
      act(() => {
        result.current.handleDeleteClick();
      });

      expect(result.current.showConfirmModal).toBe(true);

      // Then cancel
      act(() => {
        result.current.handleDeleteCancel();
      });

      expect(result.current.showConfirmModal).toBe(false);
      expect(setShowDetailModal).toHaveBeenLastCalledWith(true);
    });
  });

  describe("confirmDelete", () => {
    it("should close confirm modal and call the delete callback", () => {
      const setShowDetailModal = vi.fn();
      const onDelete = vi.fn();
      const { result } = renderHook(() =>
        useDeleteConfirmation(setShowDetailModal),
      );

      // First open the confirm modal
      act(() => {
        result.current.handleDeleteClick();
      });

      expect(result.current.showConfirmModal).toBe(true);

      // Then confirm delete
      act(() => {
        result.current.confirmDelete(onDelete);
      });

      expect(result.current.showConfirmModal).toBe(false);
      expect(onDelete).toHaveBeenCalledTimes(1);
    });

    it("should call the delete callback even when confirm modal was not open", () => {
      const setShowDetailModal = vi.fn();
      const onDelete = vi.fn();
      const { result } = renderHook(() =>
        useDeleteConfirmation(setShowDetailModal),
      );

      act(() => {
        result.current.confirmDelete(onDelete);
      });

      expect(onDelete).toHaveBeenCalledTimes(1);
    });
  });

  it("should update handleDeleteClick when setShowDetailModal changes", () => {
    const setShowDetailModal1 = vi.fn();
    const setShowDetailModal2 = vi.fn();

    const { result, rerender } = renderHook(
      ({ setter }) => useDeleteConfirmation(setter),
      { initialProps: { setter: setShowDetailModal1 } },
    );

    act(() => {
      result.current.handleDeleteClick();
    });

    expect(setShowDetailModal1).toHaveBeenCalledWith(false);
    expect(setShowDetailModal2).not.toHaveBeenCalled();

    rerender({ setter: setShowDetailModal2 });

    act(() => {
      result.current.handleDeleteClick();
    });

    expect(setShowDetailModal2).toHaveBeenCalledWith(false);
  });
});
