/**
 * Describes the structure of audio data processed by WavPacker
 */
export interface WavPackerAudioType {
  blob: Blob;
  url: string;
  arrayBuffer: ArrayBuffer;
}

export class WavPacker {
  static WAV_HEADER_SIZE = 44;

  /**
   * Merges two ArrayBuffers.
   * @param {ArrayBuffer} buffer1 - First buffer to merge.
   * @param {ArrayBuffer} buffer2 - Second buffer to merge.
   * @returns {ArrayBuffer} Merged ArrayBuffer.
   */
  static mergeBuffers(buffer1: ArrayBuffer, buffer2: ArrayBuffer): ArrayBuffer {
    const tmp = new Uint8Array(buffer1.byteLength + buffer2.byteLength);
    tmp.set(new Uint8Array(buffer1), 0);
    tmp.set(new Uint8Array(buffer2), buffer1.byteLength);
    return tmp.buffer;
  }

  /**
   * Converts Float32Array to PCM 16-bit format.
   * @param {Float32Array} input - Input float array representing audio data.
   * @returns {Int16Array} Output PCM 16-bit array.
   */
  static floatTo16BitPCM(input: Float32Array): Int16Array {
    const output = new Int16Array(input.length);
    for (let i = 0; i < input.length; i++) {
      const s = Math.max(-1, Math.min(1, input[i]));
      output[i] = s < 0 ? s * 0x8000 : s * 0x7fff;
    }
    return output;
  }

  /**
   * Packs audio data into WAV format with a given sample rate and audio data.
   * @param {number} sampleRate - Sample rate for the audio data.
   * @param {{bitsPerSample: number; channels: Float32Array[]; data: Int16Array}} audioData - Audio data to pack into WAV format.
   * @returns {WavPackerAudioType} - Packed audio in WAV format.
   */
  pack(
    sampleRate: number,
    audioData: {
      bitsPerSample: number;
      channels: Float32Array[];
      data: Int16Array;
    },
  ): WavPackerAudioType {
    const buffer = new ArrayBuffer(
      WavPacker.WAV_HEADER_SIZE + audioData.data.byteLength,
    );
    const view = new DataView(buffer);

    this._writeWavHeader(
      view,
      sampleRate,
      audioData.bitsPerSample,
      audioData.channels.length,
      audioData.data.byteLength,
    );
    new Int16Array(buffer, WavPacker.WAV_HEADER_SIZE).set(audioData.data);

    const blob = new Blob([view], { type: "audio/wav" });
    const url = URL.createObjectURL(blob);
    return { blob, url, arrayBuffer: buffer };
  }

  /**
   * Writes a WAV file header into the specified DataView.
   * @param {DataView} view - The DataView to write the header into.
   * @param {number} sampleRate - Sample rate of the audio data.
   * @param {number} bitsPerSample - Bits per sample in the audio data.
   * @param {number} channels - Number of channels in the audio data.
   * @param {number} dataSize - Size of the audio data in bytes.
   */
  private _writeWavHeader(
    view: DataView,
    sampleRate: number,
    bitsPerSample: number,
    channels: number,
    dataSize: number,
  ): void {
    const byteRate = (sampleRate * bitsPerSample * channels) / 8;
    const blockAlign = (bitsPerSample * channels) / 8;

    // "RIFF" chunk descriptor
    this._writeString(view, 0, "RIFF");
    view.setUint32(4, 36 + dataSize, true); // File size - 8 bytes
    this._writeString(view, 8, "WAVE"); // File format

    // "fmt " sub-chunk
    this._writeString(view, 12, "fmt ");
    view.setUint32(16, 16, true); // Subchunk1 size for PCM
    view.setUint16(20, 1, true); // Audio format (1 = PCM)
    view.setUint16(22, channels, true); // Number of channels
    view.setUint32(24, sampleRate, true); // Sample rate
    view.setUint32(28, byteRate, true); // Byte rate
    view.setUint16(32, blockAlign, true); // Block align
    view.setUint16(34, bitsPerSample, true); // Bits per sample

    // "data" sub-chunk
    this._writeString(view, 36, "data");
    view.setUint32(40, dataSize, true); // Data size
  }

  /**
   * Writes a string to a DataView at the specified offset.
   * @param {DataView} view - The DataView to write into.
   * @param {number} offset - Offset in the DataView.
   * @param {string} str - The string to write.
   */
  private _writeString(view: DataView, offset: number, str: string): void {
    for (let i = 0; i < str.length; i++) {
      view.setUint8(offset + i, str.charCodeAt(i));
    }
  }
}
