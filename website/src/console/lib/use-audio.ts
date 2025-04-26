import { useCallback, useEffect, useRef, useState } from "react";
import { WavStreamPlayer } from "./wavtools/wav_stream_player";

export function useAudio() {
  const streamPlayerRef = useRef<WavStreamPlayer | null>(null);
  const streamPlayerSampleRateRef = useRef<number | null>(null);
  const [isPlaying, setIsPlaying] = useState(false);

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
        sampleRate ?? streamPlayerSampleRateRef.current ?? 22000;

      // Recreate the player if the sample rate is different
      if (streamPlayerRef.current?.sampleRate !== desiredRate) {
        // Close the existing context if any
        void streamPlayerRef.current?.interrupt();
        streamPlayerRef.current = null;
      }

      if (!streamPlayerRef.current) {
        streamPlayerRef.current = new WavStreamPlayer({
          sampleRate: desiredRate,
        });
        await streamPlayerRef.current.connect();
      }

      streamPlayerSampleRateRef.current = desiredRate;

      return streamPlayerRef.current;
    },
    [],
  );

  const interruptPlayback = useCallback(async () => {
    const player = streamPlayerRef.current;
    if (!player) {
      return;
    }
    try {
      await player.interrupt();
      streamPlayerRef.current = null;
      setIsPlaying(false);
    } catch {
      /* ignore */
    }
  }, []);

  useEffect(() => {
    const itv = setInterval(() => {
      const player = streamPlayerRef.current;
      if (!player) {
        setIsPlaying(false);
        return;
      }
      void player.getTrackSampleOffset().then((offset) => {
        setIsPlaying(!!offset?.trackId);
      });
    }, 2000);
    return () => {
      clearInterval(itv);
    };
  }, []);

  const add16BitPCM = useCallback(
    async (arrayBuffer: ArrayBuffer | Int16Array, trackId?: string) => {
      const player = await ensureStreamPlayer();
      player.add16BitPCM(arrayBuffer, trackId);
      setIsPlaying(true);
    },
    [ensureStreamPlayer],
  );

  return { add16BitPCM, interruptPlayback, isPlaying };
}
