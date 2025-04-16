import { useCallback, useEffect, useRef } from "react";
import { WavStreamPlayer } from "./wavtools/wav_stream_player";

export function useAudio() {
  const streamPlayerRef = useRef<WavStreamPlayer | null>(null);
  const streamPlayerSampleRateRef = useRef<number | null>(null);
  const streamPlayerConnectedRef = useRef(false);

  useEffect(() => {
    return () => {
      const context = streamPlayerRef.current?.context;
      if (context && context.state !== "closed") {
        context.close().catch(() => {
          /* ignore */
        });
      }
    };
  }, []);

  const ensureStreamPlayer = useCallback(
    async (sampleRate?: number): Promise<WavStreamPlayer> => {
      const desiredRate =
        sampleRate ?? streamPlayerSampleRateRef.current ?? 44100;

      if (
        streamPlayerRef.current &&
        streamPlayerSampleRateRef.current !== null &&
        streamPlayerSampleRateRef.current !== desiredRate
      ) {
        const context = streamPlayerRef.current.context;
        if (context && context.state !== "closed") {
          await context.close().catch(() => {
            /* ignore */
          });
        }
        streamPlayerRef.current = null;
        streamPlayerConnectedRef.current = false;
      }

      if (!streamPlayerRef.current) {
        streamPlayerRef.current = new WavStreamPlayer({
          sampleRate: desiredRate,
        });
        streamPlayerSampleRateRef.current = desiredRate;
        streamPlayerConnectedRef.current = false;
      }

      if (!streamPlayerConnectedRef.current) {
        await streamPlayerRef.current.connect();
        streamPlayerConnectedRef.current = true;
      }

      return streamPlayerRef.current;
    },
    [],
  );

  return { ensureStreamPlayer };
}
