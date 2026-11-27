"use client";
import { useEffect, useRef } from "react";
import { Button } from "@/components/ui/button";
import { campaignCopy as c } from "@/lib/copy/campaign";
export function hoursInRange(
  values: number[],
  start: number,
  end: number,
  selected: boolean,
) {
  const next = new Set(values);
  for (let hour = Math.min(start, end); hour <= Math.max(start, end); hour++) {
    if (selected) next.add(hour);
    else next.delete(hour);
  }
  return [...next].sort((a, b) => a - b);
}
export function HourGrid({
  value,
  onChange,
  disabled = false,
}: {
  value: number[];
  onChange: (value: number[]) => void;
  disabled?: boolean;
}) {
  const group = useRef<HTMLDivElement>(null);
  const drag = useRef<{
    start: number;
    selected: boolean;
    before: number[];
  } | null>(null);
  useEffect(() => {
    const stop = () => {
      drag.current = null;
    };
    window.addEventListener("pointerup", stop);
    window.addEventListener("pointercancel", stop);
    return () => {
      window.removeEventListener("pointerup", stop);
      window.removeEventListener("pointercancel", stop);
    };
  }, []);
  return (
    <div>
      <div className="hour-shortcuts">
        {[
          [c.all, Array.from({ length: 24 }, (_, i) => i)],
          [c.businessHours, Array.from({ length: 8 }, (_, i) => i + 9)],
          [c.evenings, [18, 19, 20, 21, 22, 23]],
          [c.clear, []],
        ].map(([label, hours]) => (
          <Button
            key={String(label)}
            type="button"
            variant="outline"
            size="sm"
            disabled={disabled}
            onClick={() => onChange(hours as number[])}
          >
            {label}
          </Button>
        ))}
      </div>
      <div ref={group} className="hour-grid" role="group" aria-label={c.hours}>
        {Array.from({ length: 24 }, (_, hour) => (
          <button
            data-hour={hour}
            type="button"
            key={hour}
            disabled={disabled}
            aria-label={`${String(hour).padStart(2, "0")}:00`}
            aria-pressed={value.includes(hour)}
            onPointerDown={(event) => {
              if (disabled || event.button !== 0) return;
              event.preventDefault();
              event.currentTarget.focus();
              drag.current = {
                start: hour,
                selected: !value.includes(hour),
                before: value,
              };
              onChange(hoursInRange(value, hour, hour, !value.includes(hour)));
            }}
            onPointerEnter={() => {
              if (drag.current && !disabled)
                onChange(
                  hoursInRange(
                    drag.current.before,
                    drag.current.start,
                    hour,
                    drag.current.selected,
                  ),
                );
            }}
            onClick={(event) => {
              if (event.detail === 0)
                onChange(
                  value.includes(hour)
                    ? value.filter((item) => item !== hour)
                    : [...value, hour].sort((a, b) => a - b),
                );
            }}
            onKeyDown={(event) => {
              const offset: Record<string, number> = {
                ArrowRight: 1,
                ArrowLeft: -1,
                ArrowDown: 6,
                ArrowUp: -6,
              };
              const delta = offset[event.key];
              if (
                delta !== undefined ||
                event.key === "Home" ||
                event.key === "End"
              ) {
                event.preventDefault();
                const next =
                  event.key === "Home"
                    ? 0
                    : event.key === "End"
                      ? 23
                      : (hour + (delta ?? 0) + 24) % 24;
                group.current
                  ?.querySelectorAll<HTMLButtonElement>("[data-hour]")
                  [next]?.focus();
              }
            }}
          >
            {String(hour).padStart(2, "0")}
          </button>
        ))}
      </div>
      <small>
        {value.length ? `${value.length} ${c.selected}` : c.anyHour}.{" "}
        {c.hoursHelp}
      </small>
    </div>
  );
}
