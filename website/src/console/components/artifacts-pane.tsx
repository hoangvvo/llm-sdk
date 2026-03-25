import { diffWordsWithSpace, type Change } from "diff";
import { useMemo, useState } from "react";
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
  const [selectionState, setSelectionState] = useState<{
    list: Artifact[];
    selectedId: string | null;
    contentById: Map<string, string>;
  }>({
    list,
    selectedId: list[0]?.id ?? null,
    contentById: new Map(
      list.map((artifact) => [artifact.id, artifact.content]),
    ),
  });

  if (selectionState.list !== list) {
    let updatedId: string | null = null;
    for (const artifact of list) {
      const previousContent = selectionState.contentById.get(artifact.id);
      if (
        previousContent === undefined ||
        previousContent !== artifact.content
      ) {
        updatedId = artifact.id;
      }
    }

    let nextSelectedId = selectionState.selectedId;
    if (list.length === 0) {
      nextSelectedId = null;
    } else if (!nextSelectedId) {
      nextSelectedId = list[0].id;
    } else if (updatedId && updatedId !== nextSelectedId) {
      nextSelectedId = updatedId;
    }

    setSelectionState({
      list,
      selectedId: nextSelectedId,
      contentById: new Map(
        list.map((artifact) => [artifact.id, artifact.content]),
      ),
    });
  }

  const selectedId = selectionState.selectedId;
  const selected = useMemo(
    () => list.find((a) => a.id === selectedId) ?? list[0],
    [list, selectedId],
  );

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
                  setSelectionState((prev) => ({
                    ...prev,
                    selectedId: a.id,
                  }));
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
  const [diffState, setDiffState] = useState<{
    artifactId: string | null;
    artifactContent: string;
    contentById: Record<string, string>;
    lastDiffTokensMap: Record<string, Change[] | null>;
  }>({
    artifactId: null,
    artifactContent: "",
    contentById: {},
    lastDiffTokensMap: {},
  });

  if (
    artifact?.id &&
    (diffState.artifactId !== artifact.id ||
      diffState.artifactContent !== artifact.content)
  ) {
    const previousContent = diffState.contentById[artifact.id] ?? "";
    const nextContentById = {
      ...diffState.contentById,
      [artifact.id]: artifact.content,
    };
    const nextLastDiffTokensMap =
      artifact.content !== previousContent
        ? {
            ...diffState.lastDiffTokensMap,
            [artifact.id]: diffWordsWithSpace(
              previousContent,
              artifact.content,
            ),
          }
        : diffState.lastDiffTokensMap;

    setDiffState({
      artifactId: artifact.id,
      artifactContent: artifact.content,
      contentById: nextContentById,
      lastDiffTokensMap: nextLastDiffTokensMap,
    });
  }

  if (!artifact?.id && diffState.artifactId !== null) {
    setDiffState((prev) => ({
      ...prev,
      artifactId: null,
      artifactContent: "",
    }));
  }

  const lastDiffTokensMap = diffState.lastDiffTokensMap;
  const lastDiffTokens = lastDiffTokensMap[artifact?.id ?? ""] ?? null;

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
