import { diffWordsWithSpace, type Change } from "diff";
import { useEffect, useMemo, useRef, useState } from "react";
import type { Artifact } from "../types.ts";
import { Markdown } from "./markdown.tsx";

export function ArtifactsPane({
  artifacts,
  onDelete,
}: {
  artifacts: Artifact[] | undefined;
  onDelete?: (id: string) => void;
}) {
  const list = useMemo(() => artifacts ?? [], [artifacts]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const selected = useMemo(
    () => list.find((a) => a.id === selectedId) ?? list[0],
    [list, selectedId],
  );
  const prevMapRef = useRef<Map<string, string>>(new Map());

  // auto-select updated or newly created document, and prime prev map
  useEffect(() => {
    const prevMap = prevMapRef.current;
    let updatedId: string | null = null;
    for (const a of list) {
      const prev = prevMap.get(a.id);
      if (prev === undefined) {
        // Newly created
        updatedId = a.id;
      } else if (prev !== a.content) {
        updatedId = a.id;
      }
    }
    if (!selectedId && list.length > 0) {
      setSelectedId(list[0].id);
    } else if (updatedId && updatedId !== selectedId) {
      setSelectedId(updatedId);
    }
    // update map
    const next = new Map<string, string>();
    for (const a of list) next.set(a.id, a.content);
    prevMapRef.current = next;
  }, [list, selectedId]);

  if (list.length === 0) {
    return (
      <div className="console-card min-h-0 flex-1 overflow-hidden p-4 text-sm text-slate-600">
        <div className="font-semibold text-slate-800">Artifacts</div>
        <div className="mt-2">
          No artifacts yet. Ask the agent to draft a document and it will appear
          here.
        </div>
      </div>
    );
  }

  return (
    <div className="console-card flex min-h-0 flex-1 flex-col overflow-hidden p-3">
      <div className="mb-2 flex items-center justify-between">
        <div className="font-semibold text-slate-800">Artifacts</div>
        <div className="text-xs text-slate-500">
          {list.length} item{list.length === 1 ? "" : "s"}
        </div>
      </div>
      <div className="overflow-x-auto overflow-y-hidden whitespace-nowrap">
        <div className="flex">
          {list.map((a) => {
            const isActive = selected.id === a.id;
            return (
              <button
                key={a.id}
                className={`${
                  isActive
                    ? "border-slate-200 bg-white text-slate-900"
                    : "border-slate-200 bg-slate-100 text-slate-600 hover:bg-slate-200"
                } mr-1 border px-3 py-1 text-xs`}
                onClick={() => {
                  setSelectedId(a.id);
                }}
                title={a.id}
              >
                {a.title}
              </button>
            );
          })}
        </div>
      </div>
      <ArtifactViewer artifact={selected} onDelete={onDelete} />
    </div>
  );
}

function ArtifactViewer({
  artifact,
  onDelete,
}: {
  artifact: Artifact | undefined;
  onDelete?: (id: string) => void;
}) {
  const [showDiff, setShowDiff] = useState(false);
  const prevContentMapRef = useRef<Map<string, string>>(new Map());
  const [lastDiffTokensMap, setLastDiffTokensMap] = useState<
    Record<string, Change[] | null>
  >({});
  const lastDiffTokens = lastDiffTokensMap[artifact?.id ?? ""] ?? null;

  useEffect(() => {
    if (!artifact?.id) return;
    const curr = artifact?.content ?? "";
    const prev = prevContentMapRef.current.get(artifact.id) ?? "";
    if (curr !== prev) {
      const tokens = diffWordsWithSpace(prev, curr);
      setLastDiffTokensMap((prev) => ({
        ...prev,
        [artifact.id]: tokens,
      }));
      prevContentMapRef.current.set(artifact.id, curr);
    }
  }, [artifact?.content, artifact?.id]);

  return (
    <div className="flex min-h-0 flex-1 flex-col border border-slate-200 bg-white px-3 py-1">
      <div className="mb-2">
        <div className="truncate font-semibold text-slate-800">
          {artifact?.title}
        </div>
        <div className="truncate text-xs text-slate-500">
          {artifact?.updated_at ?? ""}
        </div>
      </div>
      <div className="min-h-0 flex-1 overflow-auto">
        {showDiff ? (
          <pre className="text-sm whitespace-pre-wrap">
            {lastDiffTokens
              ? lastDiffTokens.map((tok, i) => (
                  <span
                    key={i}
                    className={
                      tok.added
                        ? "bg-green-100 text-green-800"
                        : tok.removed
                          ? "bg-red-100 text-red-800 line-through"
                          : "text-slate-700"
                    }
                  >
                    {tok.value}
                  </span>
                ))
              : null}
          </pre>
        ) : artifact?.kind === "markdown" ? (
          <Markdown>{artifact.content}</Markdown>
        ) : (
          <pre className="text-sm whitespace-pre-wrap text-slate-800">
            {artifact?.content ?? ""}
          </pre>
        )}
      </div>
      <div className="mt-0.5 flex justify-end gap-2">
        {lastDiffTokens && (
          <button
            className={`rounded border border-amber-300 px-2 py-0.5 text-xs ${
              showDiff
                ? "border-green-300 bg-green-50 text-green-700"
                : "bg-amber-50 text-amber-700 hover:bg-amber-100"
            }`}
            onClick={() => {
              setShowDiff((v) => !v);
            }}
          >
            {showDiff ? "Hide changes" : "Show changes"}
          </button>
        )}
        {artifact ? (
          <button
            className="rounded border border-slate-300 bg-white px-2 py-0.5 text-xs text-slate-600 hover:bg-slate-50"
            onClick={() => onDelete?.(artifact.id)}
            title="Delete document"
          >
            Delete
          </button>
        ) : null}
      </div>
    </div>
  );
}
