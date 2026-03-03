"""FastAPI sidecar for LoRA adapter selection via semantic similarity (fastembed)."""

import os

from fastapi import FastAPI
from pydantic import BaseModel

from selector import pick_best

app = FastAPI(title="adapter-selection-fastembed-sidecar")


class Candidate(BaseModel):
    model_id: str
    descriptions: list[str]


class PickRequest(BaseModel):
    prompt: str
    candidates: list[Candidate]


class PickResponse(BaseModel):
    model_id: str
    score: float


@app.post("/pick", response_model=PickResponse)
def pick(req: PickRequest) -> PickResponse:
    """Select the best LoRA adapter for the given prompt.

    Returns the best candidate and its score. Thresholding is handled
    by the Go caller, not here.
    """
    candidates = [c.model_dump() for c in req.candidates]
    model_id, score = pick_best(req.prompt, candidates)

    return PickResponse(
        model_id=model_id or "",
        score=score,
    )


@app.get("/health")
def health() -> dict:
    return {"status": "ok"}


if __name__ == "__main__":
    import uvicorn

    port = int(os.environ.get("PORT", "8000"))
    # Bind 0.0.0.0: containers must bind all interfaces to be reachable
    # from pod-local traffic (the Go process connects via localhost).
    uvicorn.run(app, host="0.0.0.0", port=port)
