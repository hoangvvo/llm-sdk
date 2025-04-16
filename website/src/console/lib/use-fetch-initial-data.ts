import { useCallback, useEffect, useRef, useState } from "react";

interface UseFetchInitialDataResult<T> {
  data: T | null;
  error: string | null;
  refetch: () => void;
}

export function useFetchInitialData<T>(
  fetcher: (signal: AbortSignal) => Promise<T>,
): UseFetchInitialDataResult<T> {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  const fetcherRef = useRef(fetcher);
  const fetchIdRef = useRef(0);
  const abortControllerRef = useRef<AbortController | null>(null);

  useEffect(() => {
    fetcherRef.current = fetcher;
  }, [fetcher]);

  const runFetch = useCallback(() => {
    abortControllerRef.current?.abort();
    const abortController = new AbortController();
    abortControllerRef.current = abortController;

    const fetchId = ++fetchIdRef.current;

    setError(null);
    setData(null);

    fetcherRef
      .current(abortController.signal)
      .then((result) => {
        if (fetchIdRef.current !== fetchId) {
          return;
        }
        setData(result);
      })
      .catch((error: unknown) => {
        if (fetchIdRef.current !== fetchId) {
          return;
        }
        if (error instanceof DOMException && error.name === "AbortError") {
          return;
        }
        setError(error instanceof Error ? error.message : "Request failed");
      });
  }, []);

  useEffect(() => {
    runFetch();
    return () => {
      abortControllerRef.current?.abort();
    };
  }, [runFetch]);

  return { data, error, refetch: runFetch };
}
