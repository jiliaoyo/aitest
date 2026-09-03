"""DeepSeek-OCR 载入兼容层；模型版本必须保持 mlx-vlm 0.3.10。"""
from __future__ import annotations

from mlx_vlm import generate, load
from mlx_vlm.models.deepseekocr.processing_deepseekocr import DeepseekOCRProcessor
from mlx_vlm.tokenizer_utils import BPEStreamingDetokenizer
from transformers import AutoTokenizer
from mlx_vlm import utils


def load_ocr():
    snapshot = __import__("pathlib").Path.home() / ".cache/huggingface/hub/models--mlx-community--DeepSeek-OCR-8bit/snapshots/0e2b0e49226b5d9efc4799f4c5b1a5f423a90178"
    original_add_token = BPEStreamingDetokenizer.add_token

    def safe_add_token(self, token, skip_special_token_ids=()):
        try:
            original_add_token(self, token, skip_special_token_ids)
        except KeyError:
            self._unflushed = ""

    BPEStreamingDetokenizer.add_token = safe_add_token
    tokenizer = AutoTokenizer.from_pretrained(snapshot, trust_remote_code=False)
    processor = DeepseekOCRProcessor(
        tokenizer=tokenizer,
        candidate_resolutions=[(1024, 1024)],
        patch_size=16,
        downsample_ratio=4,
        add_special_token=False,
        mask_prompt=False,
    )
    detokenizer_class = utils.load_tokenizer(snapshot, return_tokenizer=False)
    processor.detokenizer = detokenizer_class(
        getattr(processor, "tokenizer", tokenizer)
    )
    model, _ = load("mlx-community/DeepSeek-OCR-8bit")
    processor.load_processor = lambda: processor
    tokenizer.stopping_criteria = utils.StoppingCriteria(
        getattr(model.config, "eos_token_id", None)
    )
    return model, processor, generate
