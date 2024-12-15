export interface GoogleModelOptions {
  apiKey: string;
  modelId: string;
  /**
   * If the AudioPart has a transcript, convert it to an TextPart
   */
  convertAudioPartsToTextParts?: boolean;
}
