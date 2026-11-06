# Public speaker fixtures

Three short English clips used by sherpa-onnx's speaker-verification scripts. They are not Daywire recordings and they are not a call capture.

| File | SHA-256 | Samples |
|---|---|---|
| `speaker1_a_en_16k.wav` | `cb35bff3dac9aec36e259461fecae1e1bc2ec029615f30713111cd598993676c` | 16 kHz mono PCM, 7.80 s. Enrollment. |
| `speaker1_b_en_16k.wav` | `d7daff767e13d9a2187b676d958065121cd5e26da046d65cd9604e91a87525a2` | 16 kHz mono PCM, 6.28 s. Same speaker as the enrollment, held out of the enroll file. |
| `speaker2_a_en_16k.wav` | `a723c134978a17fe12ca2374d0281a8003a56fa44ff9d2249a08791714983362` | 16 kHz mono PCM, 7.80 s. The other speaker in the mix. |

Source mirror: <https://github.com/csukuangfj/sr-data/tree/main/test/3d-speaker> (`speaker1_a_en_16k.wav`, `speaker1_b_en_16k.wav`, `speaker2_a_en_16k.wav`).

That directory's README says the files come from the ModelScope CAM++ English VoxCeleb demo set: <https://www.modelscope.cn/models/damo/speech_campplus_sv_en_voxceleb_16k/files> (the `damo` org was later renamed `iic`). sherpa-onnx downloads the same three files in `scripts/3dspeaker/run.sh` and `scripts/nemo/speaker-verification/run.sh`.

Licence: `csukuangfj/sr-data` does not add a licence file. The clips are the short VoxCeleb-derived demonstration utterances already published for those scripts. VoxCeleb is the Oxford VGG dataset of speech from YouTube interviews (<https://www.robots.ox.ac.uk/~vgg/data/voxceleb/>). The dataset page we checked does not include a separate licence grant in the page text. This directory keeps only these three files (about 200 KB each), not a VoxCeleb dump.

The demo mix is not stored. The test builds it in memory: `speaker1_b_en_16k.wav`, 0.5 s of silence, then `speaker2_a_en_16k.wav`.
