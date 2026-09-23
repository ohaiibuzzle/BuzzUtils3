"""NSFW image classifier for BuzzUtils3.

Runs the TFLite model from the old BuzzUtilityBot behind a tiny HTTP API, since
TFLite has no usable Go bindings. The bot POSTs image bytes to /classify.
"""

import io
import logging
import os
import threading
from contextlib import asynccontextmanager

import numpy as np
from ai_edge_litert.interpreter import Interpreter
from fastapi import FastAPI, File, HTTPException, UploadFile
from PIL import Image, UnidentifiedImageError

MODEL_PATH = os.environ.get("MODEL_PATH", "/app/runtime/model.tflite")
IMAGE_DIM = int(os.environ.get("IMAGE_DIM", "224"))
TFLITE_THREADS = int(os.environ.get("TFLITE_THREADS", "1"))
MAX_UPLOAD_BYTES = int(os.environ.get("MAX_UPLOAD_BYTES", str(20 * 1024 * 1024)))

# Output order of the model
CATEGORIES = ("drawings", "hentai", "neutral", "porn", "sexy")

logging.basicConfig(
    level=os.environ.get("LOG_LEVEL", "INFO"),
    format="%(asctime)s - %(name)s - %(levelname)s: %(message)s",
)
log = logging.getLogger("classifier")


class Model:
    def __init__(self, path: str):
        log.info("Loading NSFW model from %s", path)
        self.interpreter = Interpreter(model_path=path, num_threads=TFLITE_THREADS)
        self.interpreter.allocate_tensors()
        self.input = self.interpreter.get_input_details()[0]
        self.output = self.interpreter.get_output_details()[0]
        # The interpreter isn't thread-safe, and FastAPI runs sync endpoints in a thread pool
        self.lock = threading.Lock()

    def predict(self, image: Image.Image) -> dict[str, float]:
        # Same preprocessing as the old bot: nearest-neighbour resize, RGB, scaled to [0, 1]
        image = image.resize((IMAGE_DIM, IMAGE_DIM), resample=Image.Resampling.NEAREST)
        if image.mode != "RGB":
            image = image.convert("RGB")
        tensor = np.asarray(image, dtype=np.float32)[np.newaxis, ...] / 255.0

        with self.lock:
            self.interpreter.set_tensor(self.input["index"], tensor.astype(self.input["dtype"]))
            self.interpreter.invoke()
            scores = self.interpreter.get_tensor(self.output["index"])[0]

        return {name: float(score) for name, score in zip(CATEGORIES, scores)}


model: Model | None = None


@asynccontextmanager
async def lifespan(_: FastAPI):
    global model
    model = Model(MODEL_PATH)
    yield


app = FastAPI(title="BuzzUtils3 classifier", lifespan=lifespan)


@app.get("/healthz")
def healthz():
    return {"status": "ok"}


@app.post("/classify")
def classify(file: UploadFile = File(...)) -> dict[str, float]:
    data = file.file.read(MAX_UPLOAD_BYTES + 1)
    if len(data) > MAX_UPLOAD_BYTES:
        raise HTTPException(status_code=413, detail="Image is too large")

    try:
        image = Image.open(io.BytesIO(data))
        image.load()
    except (UnidentifiedImageError, OSError, Image.DecompressionBombError) as e:
        raise HTTPException(status_code=422, detail=f"Not a supported image: {e}")

    return model.predict(image)
