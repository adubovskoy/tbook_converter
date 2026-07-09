#!/bin/sh
# Creates .venv-embalign with the CPU-only dependencies for tools/embalign.py
# (the local embedding word aligner used by --align-mode emb|hybrid).
# Prefers uv (fast, can pin the Python version); falls back to python3 -m venv.
set -e
cd "$(dirname "$0")/.."

VENV=.venv-embalign
if command -v uv >/dev/null 2>&1; then
    uv venv "$VENV" --python 3.12
    uv pip install --python "$VENV/bin/python" --index-url https://download.pytorch.org/whl/cpu torch
    uv pip install --python "$VENV/bin/python" transformers numpy
else
    python3 -m venv "$VENV"
    "$VENV/bin/pip" install --index-url https://download.pytorch.org/whl/cpu torch
    "$VENV/bin/pip" install transformers numpy
fi

"$VENV/bin/python" -c "import torch, transformers; print('ok: torch', torch.__version__, '/ transformers', transformers.__version__)"
echo "Done. convert picks up $VENV/bin/python automatically (or set EMBALIGN_PYTHON)."
echo "The first --align-mode emb|hybrid run downloads the LaBSE model (~1.8 GB) into ~/.cache/huggingface."
