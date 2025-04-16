/* eslint-disable */
import type { WavPackerAudioType } from "./wav_packer.ts";
import { WavPacker } from "./wav_packer.ts";
import { AudioProcessorSrc } from "./worklets/audio_processor.ts";

/**
 * Decodes audio into a wav file
 * @typedef {Object} DecodedAudioType
 * @property {Blob} blob
 * @property {string} url
 * @property {Float32Array} values
 * @property {AudioBuffer} audioBuffer
 */
interface DecodedAudioType {
  blob: Blob;
  url: string;
  values: Float32Array;
  audioBuffer: AudioBuffer;
}

type AudioBufferData = Int16Array | ArrayBuffer;

/**
 * Records live stream of user audio as PCM16 "audio/wav" data
 */
export class WavRecorder {
  private sampleRate: number;

  private outputToSpeakers: boolean;

  private debug: boolean;

  private stream: MediaStream | null = null;

  private processor: AudioWorkletNode | null = null;

  private source: MediaStreamAudioSourceNode | null = null;

  private analyser: AnalyserNode | null = null;

  private node: AudioNode | null = null;

  private recording = false;

  private eventReceipts: Record<number, any> = {};

  private eventTimeout = 5000;

  private _chunkProcessor: (data: {
    mono: AudioBufferData;
    raw: AudioBufferData;
  }) => void = () => {};

  private _chunkProcessorSize: number | undefined;

  private _chunkProcessorBuffer = {
    raw: new ArrayBuffer(0),
    mono: new ArrayBuffer(0),
  };

  private scriptSrc: string;

  private _lastEventId = 0;

  constructor({
    sampleRate = 44100,
    outputToSpeakers = false,
    debug = false,
  }: {
    sampleRate?: number;
    outputToSpeakers?: boolean;
    debug?: boolean;
  } = {}) {
    this.scriptSrc = AudioProcessorSrc;
    this.sampleRate = sampleRate;
    this.outputToSpeakers = outputToSpeakers;
    this.debug = debug;
  }

  /**
   * Decodes audio data from multiple formats to a Blob, url, Float32Array and AudioBuffer
   */
  static async decode(
    audioData: Blob | Float32Array | Int16Array | ArrayBuffer | number[],
    sampleRate = 44100,
    fromSampleRate = -1,
  ): Promise<DecodedAudioType> {
    const context = new AudioContext({ sampleRate });
    let arrayBuffer: ArrayBuffer;
    let blob: Blob;

    if (audioData instanceof Blob) {
      if (fromSampleRate !== -1)
        throw new Error(
          `Cannot specify "fromSampleRate" when reading from Blob`,
        );
      blob = audioData;
      arrayBuffer = await blob.arrayBuffer();
    } else if (audioData instanceof ArrayBuffer) {
      if (fromSampleRate !== -1)
        throw new Error(
          `Cannot specify "fromSampleRate" when reading from ArrayBuffer`,
        );
      arrayBuffer = audioData;
      blob = new Blob([arrayBuffer], { type: "audio/wav" });
    } else {
      let float32Array: Float32Array;
      let data: Int16Array | undefined;

      if (audioData instanceof Int16Array) {
        data = audioData;
        float32Array = new Float32Array(audioData.length);
        for (let i = 0; i < audioData.length; i++) {
          float32Array[i] = audioData[i] / 0x8000;
        }
      } else if (audioData instanceof Float32Array) {
        float32Array = audioData;
      } else if (Array.isArray(audioData)) {
        float32Array = new Float32Array(audioData);
      } else {
        throw new Error(
          `"audioData" must be one of: Blob, Float32Array, Int16Array, ArrayBuffer, Array<number>`,
        );
      }

      if (fromSampleRate === -1)
        throw new Error(
          `Must specify "fromSampleRate" when reading from Float32Array, Int16Array or Array`,
        );
      if (fromSampleRate < 3000)
        throw new Error(`Minimum "fromSampleRate" is 3000 (3kHz)`);

      data = data ?? WavPacker.floatTo16BitPCM(float32Array);
      const audio = { bitsPerSample: 16, channels: [float32Array], data };
      const packer = new WavPacker();
      const result = packer.pack(fromSampleRate, audio);

      blob = result.blob;
      arrayBuffer = await blob.arrayBuffer();
    }

    const audioBuffer = await context.decodeAudioData(arrayBuffer);
    const values = audioBuffer.getChannelData(0);
    const url = URL.createObjectURL(blob);
    return { blob, url, values, audioBuffer };
  }

  private log(...args: any[]): true {
    if (this.debug) console.log(...args);
    return true;
  }

  async requestPermission(): Promise<true> {
    const permissionStatus = await navigator.permissions.query({
      name: "microphone" as PermissionName,
    });
    if (permissionStatus.state === "denied") {
      alert("You must grant microphone access to use this feature.");
    } else if (permissionStatus.state === "prompt") {
      try {
        const stream = await navigator.mediaDevices.getUserMedia({
          audio: true,
        });
        stream.getTracks().forEach((track) => {
          track.stop();
        });
      } catch {
        alert("You must grant microphone access to use this feature.");
      }
    }
    return true;
  }

  async begin(deviceId?: string): Promise<true> {
    if (this.processor)
      throw new Error(
        "Already connected: please call .end() to start a new session",
      );

    const config: MediaStreamConstraints = {
      audio: deviceId ? { deviceId: { exact: deviceId } } : true,
    };
    try {
      this.stream = await navigator.mediaDevices.getUserMedia(config);
    } catch {
      throw new Error("Could not start media stream");
    }

    const context = new AudioContext({ sampleRate: this.sampleRate });
    this.source = context.createMediaStreamSource(this.stream);

    try {
      await context.audioWorklet.addModule(this.scriptSrc);
    } catch (e) {
      console.error(e);
      throw new Error(`Could not add audioWorklet module: ${this.scriptSrc}`);
    }

    this.processor = new AudioWorkletNode(context, "audio_processor");
    this.processor.port.onmessage = (e) => {
      this._handlePortMessage(e);
    };

    const node = this.source.connect(this.processor);
    this.analyser = context.createAnalyser();
    this.analyser.fftSize = 8192;
    this.analyser.smoothingTimeConstant = 0.1;
    node.connect(this.analyser);

    if (this.outputToSpeakers) {
      console.warn(
        "Warning: Output to speakers may affect sound quality; use only for debugging",
      );
      this.analyser.connect(context.destination);
    }

    this.node = node;
    return true;
  }

  private _handlePortMessage(e: MessageEvent) {
    const { event, id, data } = e.data;
    if (event === "receipt") {
      this.eventReceipts[id] = data;
    } else if (event === "chunk") {
      if (this._chunkProcessorSize) {
        const buffer = this._chunkProcessorBuffer;
        this._chunkProcessorBuffer = {
          raw: WavPacker.mergeBuffers(buffer.raw, data.raw),
          mono: WavPacker.mergeBuffers(buffer.mono, data.mono),
        };
        if (
          this._chunkProcessorBuffer.mono.byteLength >= this._chunkProcessorSize
        ) {
          this._chunkProcessor(this._chunkProcessorBuffer);
          this._chunkProcessorBuffer = {
            raw: new ArrayBuffer(0),
            mono: new ArrayBuffer(0),
          };
        }
      } else {
        this._chunkProcessor(data);
      }
    }
  }

  async record(
    chunkProcessor: (data: {
      mono: AudioBufferData;
      raw: AudioBufferData;
    }) => any = () => {},
    chunkSize = 8192,
  ): Promise<true> {
    if (!this.processor)
      throw new Error("Session ended: please call .begin() first");
    if (this.recording)
      throw new Error("Already recording: please call .pause() first");

    this._chunkProcessor = chunkProcessor;
    this._chunkProcessorSize = chunkSize;
    this._chunkProcessorBuffer = {
      raw: new ArrayBuffer(0),
      mono: new ArrayBuffer(0),
    };

    this.log("Recording ...");
    await this._event("start");
    this.recording = true;
    return true;
  }

  async pause(): Promise<true> {
    if (!this.processor)
      throw new Error("Session ended: please call .begin() first");
    if (!this.recording)
      throw new Error("Already paused: please call .record() first");

    if (this._chunkProcessorBuffer.raw.byteLength)
      this._chunkProcessor(this._chunkProcessorBuffer);
    this.log("Pausing ...");
    await this._event("stop");
    this.recording = false;
    return true;
  }

  async end(): Promise<WavPackerAudioType> {
    if (!this.processor)
      throw new Error("Session ended: please call .begin() first");

    this.log("Stopping ...");
    await this._event("stop");
    this.recording = false;
    this.stream?.getTracks().forEach((track) => {
      track.stop();
    });

    this.log("Exporting ...");
    const exportData = await this._event("export", {}, this.processor);

    this.processor.disconnect();
    this.source?.disconnect();
    this.node?.disconnect();
    this.analyser?.disconnect();
    this.stream = null;
    this.processor = null;
    this.source = null;
    this.node = null;

    const packer = new WavPacker();
    return packer.pack(this.sampleRate, exportData.audio);
  }

  private async _event(
    name: string,
    data: Record<string, any> = {},
    _processor: AudioWorkletNode | null = null,
  ): Promise<Record<string, any>> {
    _processor = _processor || this.processor;
    if (!_processor)
      throw new Error("Cannot send events without recording first");

    const message = { event: name, id: this._lastEventId++, data };
    _processor.port.postMessage(message);

    const t0 = Date.now();
    while (!this.eventReceipts[message.id]) {
      if (Date.now() - t0 > this.eventTimeout)
        throw new Error(`Timeout waiting for "${name}" event`);
      await new Promise((res) => {
        setTimeout(() => {
          res(true);
        }, 1);
      });
    }

    const payload = this.eventReceipts[message.id];
    delete this.eventReceipts[message.id];
    return payload;
  }
}
