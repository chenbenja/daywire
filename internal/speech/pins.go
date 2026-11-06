package speech

// Pinned speech files. The phone does not download them. hack/fetch-speech-models.sh
// checks these SHA-256 values at build time and copies the files into the app
// assets directory. The hashes are of the published bytes, verified in this
// repository before they were written down.
const (
	SherpaVersion = "1.12.28"
	SherpaAARURL  = "https://github.com/k2-fsa/sherpa-onnx/releases/download/v1.12.28/sherpa-onnx-1.12.28.aar"
	SherpaAARSHA  = "b948066872e698f2e37e3c936c303426e12c5b7f9280e24f18767ed2efa08f8a"
	SherpaAARFile = "sherpa-onnx-1.12.28.aar"

	// SpeakerNet and pyannote segmentation match internal/speaker/models.go.
	EmbeddingURL   = "https://github.com/k2-fsa/sherpa-onnx/releases/download/speaker-recongition-models/nemo_en_speakerverification_speakernet.onnx"
	EmbeddingSHA   = "d204dc8aac0014b8543f05fc8e310510c7022bc65b6452c203ec205ef7a66b23"
	EmbeddingFile  = "nemo_en_speakerverification_speakernet.onnx"
	EmbeddingAsset = "speech/embedding.onnx"

	SegmentationTarURL = "https://github.com/k2-fsa/sherpa-onnx/releases/download/speaker-segmentation-models/sherpa-onnx-pyannote-segmentation-3-0.tar.bz2"
	SegmentationTarSHA = "24615ee884c897d9d2ba09bb4d30da6bb1b15e685065962db5b02e76e4996488"
	SegmentationSHA    = "220ad67ca923bef2fa91f2390c786097bf305bceb5e261d4af67b38e938e1079"
	SegmentationAsset  = "speech/segmentation.onnx"

	MoonshineTarURL = "https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-moonshine-tiny-en-int8.tar.bz2"
	MoonshineTarSHA = "d5fe6ec4334fef36255b2a4010412cad4c007e33103fec62fb5d17cad88086f2"
	MoonshinePreSHA = "f33addce61a143460fe753b5ee5b7db255e5140b5b779c065b94f6c83ff0bf4e"
	MoonshineEncSHA = "8774dfba578de027ec6595c2c654a0836434489bc963a0db124a7f181f571acb"
	MoonshineUncSHA = "216737000dd5881a17aa043f6bbd286add33e4c3b0ae257153e2ec15438bdc41"
	MoonshineDecSHA = "2aff28bba6a03d8dcf5c9feac45462629bae37317442299f28115ad09da773f6"
	MoonshineTokSHA = "1165c2aeb9f72f457a83be2d459a09054f27490acd9b41bd43794dfd25e296ea"

	MoonshinePreAsset = "speech/moonshine/preprocess.onnx"
	MoonshineEncAsset = "speech/moonshine/encode.int8.onnx"
	MoonshineUncAsset = "speech/moonshine/uncached_decode.int8.onnx"
	MoonshineDecAsset = "speech/moonshine/cached_decode.int8.onnx"
	MoonshineTokAsset = "speech/moonshine/tokens.txt"

	HebrewEncURL  = "https://huggingface.co/ivrit-ai/zipformer2-66m-streaming-he/resolve/main/sherpa-onnx/encoder.int8.onnx"
	HebrewDecURL  = "https://huggingface.co/ivrit-ai/zipformer2-66m-streaming-he/resolve/main/sherpa-onnx/decoder.int8.onnx"
	HebrewJoinURL = "https://huggingface.co/ivrit-ai/zipformer2-66m-streaming-he/resolve/main/sherpa-onnx/joiner.int8.onnx"
	HebrewTokURL  = "https://huggingface.co/ivrit-ai/zipformer2-66m-streaming-he/resolve/main/sherpa-onnx/tokens.txt"
	HebrewEncSHA  = "6c6febcb74a09435315a1d872a6539f6e7fc2c333533f14c74edc0e85c49ffcd"
	HebrewDecSHA  = "feb03e7c8aa761593a9284a6fa1e91259be3702de75f630062256c1a59c5a906"
	HebrewJoinSHA = "1d2c6d8e6dabc5eb52de52e95718e9bdfbffbfe7ead3808fb5ffd74e83bd9cae"
	HebrewTokSHA  = "e4ebb5e72a6aeef47437a55fa2308ebbedb7c51c557de7a89bf965c23ea9af4a"

	HebrewEncAsset  = "speech/zipformer-he/encoder.int8.onnx"
	HebrewDecAsset  = "speech/zipformer-he/decoder.int8.onnx"
	HebrewJoinAsset = "speech/zipformer-he/joiner.int8.onnx"
	HebrewTokAsset  = "speech/zipformer-he/tokens.txt"
)

// EnglishAssets are required for enrollment-independent English processing,
// together with the embedding and segmentation models.
func EnglishAssets() []string {
	return []string{
		EmbeddingAsset,
		SegmentationAsset,
		MoonshinePreAsset,
		MoonshineEncAsset,
		MoonshineUncAsset,
		MoonshineDecAsset,
		MoonshineTokAsset,
	}
}

// HebrewAssets are the Zipformer files. English does not need them.
func HebrewAssets() []string {
	return []string{
		HebrewEncAsset,
		HebrewDecAsset,
		HebrewJoinAsset,
		HebrewTokAsset,
	}
}
