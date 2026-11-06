# LibriSpeech fixtures

Three short clips from LibriSpeech test-clean, converted from the corpus FLAC to 16 kHz mono 16-bit PCM. No other edits. They are not personal recordings.

Licence: [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/). LibriSpeech is published by Vassil Panayotov, with Guoguo Chen, Daniel Povey, and Sanjeev Khudanpur, at <https://www.openslr.org/12/> (OpenSLR page: "License: CC BY 4.0"). The readings are LibriVox audiobooks of public-domain books. Cite the corpus if you reuse the clips: Panayotov et al., "LibriSpeech: an ASR corpus based on public domain audio books", ICASSP 2015.

| File | Utterance | Role | Duration | SHA-256 |
|---|---|---|---|---|
| `librispeech-6930-75918-0000.wav` | `6930-75918-0000` | Enrollment of reader 6930 | 3.51 s | `746c756a0b42872638c5f4bb5a8e282ca113bcdaf786bc54591d745dbed96f47` |
| `librispeech-6930-75918-0001.wav` | `6930-75918-0001` | Same reader, held out of the enrollment file | 14.23 s | `6d9736c06b09ac23c50a668047f430e157edf980e79629a65d0841cd195927a8` |
| `librispeech-1320-122617-0000.wav` | `1320-122617-0000` | Different reader (1320) | 7.84 s | `1a50ccdc40469ef16df72473e4cf7502cf36afcb550b75d80d6f923ca866d955` |

The demo mix is not stored. The test builds it in memory: utterance `6930-75918-0001`, 0.5 s of silence, then `1320-122617-0000`.
