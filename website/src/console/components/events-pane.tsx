import { useCallback, useState } from "react";
import type { LoggedEvent } from "../types.ts";

interface EventsPaneProps {
  events: LoggedEvent[];
}

export function EventsPane({ events }: EventsPaneProps) {
  const [expandedIds, setExpandedIds] = useState<Set<string>>(new Set());

  const toggle = useCallback((id: string) => {
    setExpandedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }, []);

  if (events.length === 0) {
    return (
      <div className="flex-1 overflow-y-auto px-8 py-6">
        <div className="console-placeholder p-10">No events yet.</div>
      </div>
    );
  }

  return (
    <div className="flex-1 overflow-y-auto px-8 py-6">
      <div className="space-y-3">
        {events.map((event) => {
          const expanded = expandedIds.has(event.id);
          return (
            <div
              key={event.id}
              className="console-surface overflow-hidden p-0!"
            >
              <button
                type="button"
                className="flex w-full items-center justify-between px-4 py-3 text-left"
                onClick={() => {
                  toggle(event.id);
                }}
              >
                <div className="flex items-center gap-3 text-xs tracking-[0.15em] text-slate-600">
                  <span
                    className={
                      event.direction === "client"
                        ? "text-emerald-600"
                        : "text-sky-600"
                    }
                  >
                    {event.direction === "client" ? "↑" : "↓"}
                  </span>
                  <span className="font-medium text-slate-700">
                    {event.name}
                  </span>
                </div>
                <div className="text-[11px] text-slate-500">
                  {new Date(event.timestamp).toLocaleTimeString()}
                </div>
              </button>
              {expanded ? (
                <div className="border-t border-slate-200 bg-white px-4 py-3">
                  <div className="max-h-64 overflow-auto rounded-lg bg-slate-950/90 p-4 text-xs text-slate-100">
                    <pre className="overflow-x-auto whitespace-pre">
                      {JSON.stringify(event.payload, null, 2)}
                    </pre>
                  </div>
                </div>
              ) : null}
            </div>
          );
        })}
      </div>
    </div>
  );
}
