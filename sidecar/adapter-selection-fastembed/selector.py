"""Adapter selection via fastembed embeddings (ONNX-based, lightweight)."""

import os
from functools import lru_cache

import numpy as np
from fastembed import TextEmbedding


def _model_name() -> str:
    return os.environ.get(
        "EMBEDDING_MODEL", "sentence-transformers/all-MiniLM-L6-v2"
    )


@lru_cache(maxsize=1)
def _load_model() -> TextEmbedding:
    return TextEmbedding(model_name=_model_name())


# In-memory embedding cache: description text -> numpy array.
# Thread safety: Python's GIL makes dict reads/writes atomic. FastAPI sync
# endpoints run in a thread pool, but the GIL protects _embedding_cache.
_embedding_cache: dict[str, np.ndarray] = {}


def _embed(texts: list[str]) -> np.ndarray:
    """Embed texts, using cache for previously seen strings."""
    model = _load_model()
    uncached_texts: list[str] = []

    for t in texts:
        if t not in _embedding_cache:
            uncached_texts.append(t)

    if uncached_texts:
        new_embeddings = list(model.embed(uncached_texts))
        for text, emb in zip(uncached_texts, new_embeddings):
            # Normalize embedding for cosine similarity via dot product
            norm = np.linalg.norm(emb)
            _embedding_cache[text] = emb / norm if norm > 0 else emb

    result = np.array([_embedding_cache[t] for t in texts])
    return result


def pick_best(
    prompt: str,
    candidates: list[dict],
) -> tuple[str | None, float]:
    """Select the best candidate by cosine similarity.

    Args:
        prompt: The user's prompt text.
        candidates: List of dicts with 'model_id' and 'descriptions'.

    Returns:
        (model_id, score) of the best match, or (None, 0.0) if no candidates.
    """
    if not candidates:
        return None, 0.0

    # Embed prompt
    prompt_emb = _embed([prompt])[0]

    best_model_id: str | None = None
    best_score: float = -1.0

    for cand in candidates:
        descriptions = cand["descriptions"]
        if not descriptions:
            continue

        # Embed all descriptions for this candidate
        desc_embs = _embed(descriptions)

        # Cosine similarity (embeddings are normalized)
        similarities = desc_embs @ prompt_emb
        max_sim = float(np.max(similarities))

        if max_sim > best_score:
            best_score = max_sim
            best_model_id = cand["model_id"]

    return best_model_id, best_score
