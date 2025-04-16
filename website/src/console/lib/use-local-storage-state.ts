import type { Dispatch, SetStateAction } from "react";
import { useCallback, useEffect, useRef, useState } from "react";

const isBrowser = typeof window !== "undefined";

const defaultSerialize = (value: unknown): string => JSON.stringify(value);

const defaultDeserialize = (value: string): unknown => JSON.parse(value);

function resolveInitialValue<T>(value: T | (() => T)): T {
  return typeof value === "function" ? (value as () => T)() : value;
}

export function useLocalStorageState<T>(
  key: string,
  initialValue: T | (() => T),
): [T, Dispatch<SetStateAction<T>>] {
  const serialize = defaultSerialize as (value: T) => string;
  const deserialize = defaultDeserialize as (value: string) => T;

  const initialRef = useRef(initialValue);

  const readStoredValue = useCallback((): T => {
    if (!isBrowser) {
      return resolveInitialValue(initialRef.current);
    }
    try {
      const stored = window.localStorage.getItem(key);
      if (stored === null) {
        return resolveInitialValue(initialRef.current);
      }
      return deserialize(stored);
    } catch {
      return resolveInitialValue(initialRef.current);
    }
  }, [deserialize, key]);

  const [value, setValue] = useState<T>(initialRef.current);

  useEffect(() => {
    setValue(readStoredValue());
  }, [readStoredValue]);

  const setStoredValue = useCallback<Dispatch<SetStateAction<T>>>(
    (update) => {
      setValue((currentValue) => {
        const nextValue =
          typeof update === "function"
            ? (update as (previous: T) => T)(currentValue)
            : update;

        if (Object.is(nextValue, currentValue)) {
          return currentValue;
        }

        if (!isBrowser) {
          return nextValue;
        }

        try {
          const serialized = serialize(nextValue);
          window.localStorage.setItem(key, serialized);
        } catch {
          /* ignore write errors */
        }

        return nextValue;
      });
    },
    [key, serialize],
  );

  return [value, setStoredValue];
}
