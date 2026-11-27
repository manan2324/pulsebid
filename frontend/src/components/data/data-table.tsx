"use client";
import { useState } from "react";
import {
  flexRender,
  getCoreRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
  type ColumnDef,
  type SortingState,
  type VisibilityState,
} from "@tanstack/react-table";
import {
  ArrowDown,
  ArrowUp,
  ChevronsUpDown,
  SlidersHorizontal,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState, ErrorState } from "./shared";
import { ApiClientError } from "@/lib/api/client";
import { catalogCopy as c } from "@/lib/copy/catalog";
export interface DataTableProps<T> {
  columns: ColumnDef<T>[];
  data: T[];
  loading: boolean;
  error?: Error | null;
  retry: () => void;
  emptyTitle: string;
  emptyDescription: string;
  emptyAction?: React.ReactNode;
  toolbar?: React.ReactNode;
  hasMore?: boolean;
  loadingMore?: boolean;
  loadMore?: () => void;
  footerHelp?: string;
}
export function DataTable<T extends { id: string }>({
  columns,
  data,
  loading,
  error,
  retry,
  emptyTitle,
  emptyDescription,
  emptyAction,
  toolbar,
  hasMore,
  loadingMore,
  loadMore,
  footerHelp = c.filterHelp,
}: DataTableProps<T>) {
  const [sorting, setSorting] = useState<SortingState>([]);
  const [visibility, setVisibility] = useState<VisibilityState>({});
  const table = useReactTable({
    data,
    columns,
    state: { sorting, columnVisibility: visibility },
    onSortingChange: setSorting,
    onColumnVisibilityChange: setVisibility,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    initialState: { pagination: { pageSize: 20 } },
    getRowId: (row) => row.id,
  });
  return (
    <section className="panel catalog-table">
      <div className="table-toolbar">
        {toolbar}
        <details className="column-picker">
          <summary>
            <SlidersHorizontal size={14} />
            {c.columns}
          </summary>
          <div>
            {table
              .getAllLeafColumns()
              .filter((column) => column.getCanHide())
              .map((column) => (
                <label key={column.id}>
                  <input
                    type="checkbox"
                    checked={column.getIsVisible()}
                    onChange={column.getToggleVisibilityHandler()}
                  />
                  {typeof column.columnDef.header === "string"
                    ? column.columnDef.header
                    : column.id}
                </label>
              ))}
          </div>
        </details>
      </div>
      {error && (
        <ErrorState
          message={error.message}
          requestId={
            error instanceof ApiClientError ? error.requestId : undefined
          }
          retry={retry}
        />
      )}
      {loading ? (
        <div className="table-skeleton" role="status" aria-label={c.checking}>
          {Array.from({ length: 6 }, (_, index) => (
            <Skeleton key={index} className="h-10 w-full" />
          ))}
        </div>
      ) : data.length === 0 && !error ? (
        <EmptyState
          title={emptyTitle}
          description={emptyDescription}
          action={emptyAction}
        />
      ) : data.length > 0 ? (
        <div className="table-scroll">
          <table>
            <thead>
              {table.getHeaderGroups().map((group) => (
                <tr key={group.id}>
                  {group.headers.map((header) => (
                    <th
                      key={header.id}
                      scope="col"
                      aria-sort={
                        header.column.getIsSorted() === "asc"
                          ? "ascending"
                          : header.column.getIsSorted() === "desc"
                            ? "descending"
                            : undefined
                      }
                    >
                      {header.isPlaceholder ? null : header.column.getCanSort() ? (
                        <button
                          className="sort-button"
                          onClick={header.column.getToggleSortingHandler()}
                        >
                          {flexRender(
                            header.column.columnDef.header,
                            header.getContext(),
                          )}
                          {header.column.getIsSorted() === "asc" ? (
                            <ArrowUp size={12} />
                          ) : header.column.getIsSorted() === "desc" ? (
                            <ArrowDown size={12} />
                          ) : (
                            <ChevronsUpDown size={12} />
                          )}
                        </button>
                      ) : (
                        flexRender(
                          header.column.columnDef.header,
                          header.getContext(),
                        )
                      )}
                    </th>
                  ))}
                </tr>
              ))}
            </thead>
            <tbody>
              {table.getRowModel().rows.map((row) => (
                <tr key={row.id}>
                  {row.getVisibleCells().map((cell) => (
                    <td key={cell.id}>
                      {flexRender(
                        cell.column.columnDef.cell,
                        cell.getContext(),
                      )}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}
      <div className="table-footer">
        <span>
          {data.length} {c.loaded}
        </span>
        <div>
          {hasMore && (
            <Button
              variant="outline"
              size="sm"
              onClick={loadMore}
              disabled={loadingMore}
            >
              {loadingMore ? c.loadingMore : c.loadMore}
            </Button>
          )}
          <span>
            {c.page} {table.getState().pagination.pageIndex + 1} {c.of}{" "}
            {Math.max(1, table.getPageCount())}
          </span>
          <Button
            variant="outline"
            size="sm"
            onClick={() => table.previousPage()}
            disabled={!table.getCanPreviousPage()}
          >
            {c.previous}
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => table.nextPage()}
            disabled={!table.getCanNextPage()}
          >
            {c.next}
          </Button>
        </div>
      </div>
      <p className="table-help">{footerHelp}</p>
    </section>
  );
}
