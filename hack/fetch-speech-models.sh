#!/usr/bin/env bash
# Fetch the pinned sherpa-onnx Android archive and the on-device speech models.
#
# This runs on a developer machine at build time. The phone app has no
# INTERNET permission and does not download these files. Nothing here is
# committed. SHA-256 values match internal/speech/pins.go.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

aar_url="https://github.com/k2-fsa/sherpa-onnx/releases/download/v1.12.28/sherpa-onnx-1.12.28.aar"
aar_sha="b948066872e698f2e37e3c936c303426e12c5b7f9280e24f18767ed2efa08f8a"
aar_dest="android/app/libs/sherpa-onnx-1.12.28.aar"

embedding_url="https://github.com/k2-fsa/sherpa-onnx/releases/download/speaker-recongition-models/nemo_en_speakerverification_speakernet.onnx"
embedding_sha="d204dc8aac0014b8543f05fc8e310510c7022bc65b6452c203ec205ef7a66b23"

seg_tar_url="https://github.com/k2-fsa/sherpa-onnx/releases/download/speaker-segmentation-models/sherpa-onnx-pyannote-segmentation-3-0.tar.bz2"
seg_tar_sha="24615ee884c897d9d2ba09bb4d30da6bb1b15e685065962db5b02e76e4996488"
seg_sha="220ad67ca923bef2fa91f2390c786097bf305bceb5e261d4af67b38e938e1079"
seg_rel="sherpa-onnx-pyannote-segmentation-3-0/model.onnx"

moon_tar_url="https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-moonshine-tiny-en-int8.tar.bz2"
moon_tar_sha="d5fe6ec4334fef36255b2a4010412cad4c007e33103fec62fb5d17cad88086f2"
moon_pre_sha="f33addce61a143460fe753b5ee5b7db255e5140b5b779c065b94f6c83ff0bf4e"
moon_enc_sha="8774dfba578de027ec6595c2c654a0836434489bc963a0db124a7f181f571acb"
moon_unc_sha="216737000dd5881a17aa043f6bbd286add33e4c3b0ae257153e2ec15438bdc41"
moon_dec_sha="2aff28bba6a03d8dcf5c9feac45462629bae37317442299f28115ad09da773f6"
moon_tok_sha="1165c2aeb9f72f457a83be2d459a09054f27490acd9b41bd43794dfd25e296ea"

he_enc_url="https://huggingface.co/ivrit-ai/zipformer2-66m-streaming-he/resolve/main/sherpa-onnx/encoder.int8.onnx"
he_dec_url="https://huggingface.co/ivrit-ai/zipformer2-66m-streaming-he/resolve/main/sherpa-onnx/decoder.int8.onnx"
he_join_url="https://huggingface.co/ivrit-ai/zipformer2-66m-streaming-he/resolve/main/sherpa-onnx/joiner.int8.onnx"
he_tok_url="https://huggingface.co/ivrit-ai/zipformer2-66m-streaming-he/resolve/main/sherpa-onnx/tokens.txt"
he_enc_sha="6c6febcb74a09435315a1d872a6539f6e7fc2c333533f14c74edc0e85c49ffcd"
he_dec_sha="feb03e7c8aa761593a9284a6fa1e91259be3702de75f630062256c1a59c5a906"
he_join_sha="1d2c6d8e6dabc5eb52de52e95718e9bdfbffbfe7ead3808fb5ffd74e83bd9cae"
he_tok_sha="e4ebb5e72a6aeef47437a55fa2308ebbedb7c51c557de7a89bf965c23ea9af4a"

assets="android/app/speech-assets"
work="output/speech-fetch"
mkdir -p "${work}" "${assets}/speech/moonshine" "${assets}/speech/zipformer-he" "$(dirname "${aar_dest}")"

sha256_file() {
	local file="$1"
	local want="$2"
	local got
	got=$(sha256sum "${file}" | awk '{print $1}')
	if [[ "${got}" != "${want}" ]]; then
		echo "checksum mismatch for ${file}" >&2
		exit 1
	fi
}

fetch() {
	local url="$1"
	local dest="$2"
	local want="$3"
	if [[ -f "${dest}" ]]; then
		local got
		got=$(sha256sum "${dest}" | awk '{print $1}')
		if [[ "${got}" == "${want}" ]]; then
			return 0
		fi
		rm -f "${dest}"
	fi
	echo "fetch ${dest}"
	curl -fsSL --retry 3 --retry-delay 2 -o "${dest}.partial" "${url}"
	mv "${dest}.partial" "${dest}"
	sha256_file "${dest}" "${want}"
}

install_file() {
	local src="$1"
	local dest="$2"
	local want="$3"
	sha256_file "${src}" "${want}"
	cp -f "${src}" "${dest}.partial"
	mv "${dest}.partial" "${dest}"
	sha256_file "${dest}" "${want}"
}

fetch "${aar_url}" "${work}/sherpa-onnx-1.12.28.aar" "${aar_sha}"
install_file "${work}/sherpa-onnx-1.12.28.aar" "${aar_dest}" "${aar_sha}"

fetch "${embedding_url}" "${work}/embedding.onnx" "${embedding_sha}"
install_file "${work}/embedding.onnx" "${assets}/speech/embedding.onnx" "${embedding_sha}"

fetch "${seg_tar_url}" "${work}/segmentation.tar.bz2" "${seg_tar_sha}"
if [[ ! -f "${work}/segmentation.onnx" ]] || ! sha256sum "${work}/segmentation.onnx" | awk '{print $1}' | grep -qx "${seg_sha}"; then
	rm -rf "${work}/seg"
	mkdir -p "${work}/seg"
	tar -xjf "${work}/segmentation.tar.bz2" -C "${work}/seg" "${seg_rel}"
	cp -f "${work}/seg/${seg_rel}" "${work}/segmentation.onnx"
fi
install_file "${work}/segmentation.onnx" "${assets}/speech/segmentation.onnx" "${seg_sha}"

fetch "${moon_tar_url}" "${work}/moonshine.tar.bz2" "${moon_tar_sha}"
moon_dir="${work}/sherpa-onnx-moonshine-tiny-en-int8"
if [[ ! -f "${moon_dir}/tokens.txt" ]]; then
	tar -xjf "${work}/moonshine.tar.bz2" -C "${work}"
fi
# The archive also contains test wavs. Those are not packaged.
install_file "${moon_dir}/preprocess.onnx" "${assets}/speech/moonshine/preprocess.onnx" "${moon_pre_sha}"
install_file "${moon_dir}/encode.int8.onnx" "${assets}/speech/moonshine/encode.int8.onnx" "${moon_enc_sha}"
install_file "${moon_dir}/uncached_decode.int8.onnx" "${assets}/speech/moonshine/uncached_decode.int8.onnx" "${moon_unc_sha}"
install_file "${moon_dir}/cached_decode.int8.onnx" "${assets}/speech/moonshine/cached_decode.int8.onnx" "${moon_dec_sha}"
install_file "${moon_dir}/tokens.txt" "${assets}/speech/moonshine/tokens.txt" "${moon_tok_sha}"

fetch "${he_enc_url}" "${work}/he-encoder.int8.onnx" "${he_enc_sha}"
fetch "${he_dec_url}" "${work}/he-decoder.int8.onnx" "${he_dec_sha}"
fetch "${he_join_url}" "${work}/he-joiner.int8.onnx" "${he_join_sha}"
fetch "${he_tok_url}" "${work}/he-tokens.txt" "${he_tok_sha}"
install_file "${work}/he-encoder.int8.onnx" "${assets}/speech/zipformer-he/encoder.int8.onnx" "${he_enc_sha}"
install_file "${work}/he-decoder.int8.onnx" "${assets}/speech/zipformer-he/decoder.int8.onnx" "${he_dec_sha}"
install_file "${work}/he-joiner.int8.onnx" "${assets}/speech/zipformer-he/joiner.int8.onnx" "${he_join_sha}"
install_file "${work}/he-tokens.txt" "${assets}/speech/zipformer-he/tokens.txt" "${he_tok_sha}"

echo "speech models ready in ${assets} and ${aar_dest}"
