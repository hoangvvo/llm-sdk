import assert from "node:assert/strict";
import test from "node:test";
import { mapMimeTypeToAudioFormat } from "./audio-part.utils.ts";

test("maps parameterized MIME types case-insensitively", () => {
  assert.equal(
    mapMimeTypeToAudioFormat("audio/l16; rate=24000; channels=1"),
    "linear16",
  );
});
