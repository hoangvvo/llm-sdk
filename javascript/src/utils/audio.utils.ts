export function floatTo16BitPCM(float32Array: Float32Array): ArrayBuffer {
  const buffer = new ArrayBuffer(float32Array.length * 2);
  const view = new DataView(buffer);
  let offset = 0;
  for (let i = 0; i < float32Array.length; i++, offset += 2) {
    const s = Math.max(-1, Math.min(1, float32Array[i]!));
    view.setInt16(offset, s < 0 ? s * 0x8000 : s * 0x7fff, true);
  }
  return buffer;
}

export function base64ToArrayBuffer(base64: string): ArrayBuffer {
  const binaryString = atob(base64);
  const len = binaryString.length;
  const bytes = new Uint8Array(len);
  for (let i = 0; i < len; i++) {
    bytes[i] = binaryString.charCodeAt(i);
  }
  return bytes.buffer;
}

export function mergeInt16Arrays(left: unknown, right: unknown) {
  if (left instanceof ArrayBuffer) {
    left = new Int16Array(left);
  }
  if (right instanceof ArrayBuffer) {
    right = new Int16Array(right);
  }
  if (!(left instanceof Int16Array) || !(right instanceof Int16Array)) {
    throw new Error(`Both items must be Int16Array`);
  }
  const newValues = new Int16Array(left.length + right.length);
  for (let i = 0; i < left.length; i++) {
    newValues[i] = left[i]!;
  }
  for (let j = 0; j < right.length; j++) {
    newValues[left.length + j] = right[j]!;
  }
  return newValues;
}

export function arrayBufferToBase64(arrayBuffer: ArrayBuffer): string {
  if (arrayBuffer instanceof Float32Array) {
    arrayBuffer = floatTo16BitPCM(arrayBuffer);
  } else if (arrayBuffer instanceof Int16Array) {
    arrayBuffer = arrayBuffer.buffer;
  }
  let binary = "";
  const bytes = new Uint8Array(arrayBuffer);
  const chunkSize = 0x8000; // 32KB chunk size
  for (let i = 0; i < bytes.length; i += chunkSize) {
    const chunk = bytes.subarray(i, i + chunkSize);
    binary += String.fromCharCode(...chunk);
  }
  return btoa(binary);
}
