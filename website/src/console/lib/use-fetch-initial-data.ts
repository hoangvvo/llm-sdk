import {
  useCallback,
  useEffect,
  useRef,
  useState,
  useEffectEvent,
} from "react";

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
  const fetchIdRef = useRef(0);
  const abortControllerRef = useRef<AbortController | null>(null);

  const runFetch = useCallback(() => {
    abortControllerRef.current?.abort();
    const abortController = new AbortController();
    abortControllerRef.current = abortController;

    const fetchId = ++fetchIdRef.current;

    setError(null);
    setData(null);

    fetcher(abortController.signal)
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
  }, [fetcher]);

  const onInitialFetch = useEffectEvent(() => {
    runFetch();
  });

  useEffect(() => {
    onInitialFetch();
    return () => {
      abortControllerRef.current?.abort();
    };
  }, []);

  return { data, error, refetch: runFetch };
}
