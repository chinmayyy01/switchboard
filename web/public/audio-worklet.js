class PCMCapture extends AudioWorkletProcessor {
  constructor() {
    super();
    this.frameSize = 320;
    this.out = new Int16Array(this.frameSize);
    this.filled = 0;
  }

  process(inputs) {
    const input = inputs[0][0];
    if (!input) return true;

    for (let i = 0; i < input.length; i++) {
      let s = input[i];
      if (s > 1) s = 1;
      else if (s < -1) s = -1;

      this.out[this.filled++] = Math.round(s * 32767);

      if (this.filled === this.frameSize) {
        this.port.postMessage(this.out.slice(0));
        this.filled = 0;
      }
    }

    return true;
  }
}

registerProcessor("pcm-capture", PCMCapture);
