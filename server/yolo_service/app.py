import os
import io
from flask import Flask, request, jsonify
from ultralytics import YOLO
from PIL import Image

app = Flask(__name__)

MODEL_PATH = os.environ.get("YOLO_MODEL_PATH", "models/yolo11s_best.pt")
CONFIDENCE_THRESHOLD = float(os.environ.get("YOLO_CONFIDENCE", "0.5"))

CLASS_NAMES = [
    '1m', '1p', '1s', '1z', '2m', '2p', '2s', '2z',
    '3m', '3p', '3s', '3z', '4m', '4p', '4s', '4z',
    '5m', '5p', '5s', '5z', '6m', '6p', '6s', '6z',
    '7m', '7p', '7s', '7z', '8m', '8p', '8s',
    '9m', '9p', '9s', 'UNKNOWN', '0m', '0p', '0s'
]

RED_FIVE_MAP = {'0m': '5m', '0p': '5p', '0s': '5s'}

model = None


def load_model():
    global model
    if not os.path.exists(MODEL_PATH):
        return False
    model = YOLO(MODEL_PATH)
    return True


def build_tile_string(tiles):
    groups = {'m': [], 'p': [], 's': [], 'z': []}
    for t in tiles:
        num = t[0]
        suit = t[1]
        groups[suit].append(num)
    result = ''
    for suit in ['m', 'p', 's', 'z']:
        if groups[suit]:
            result += ''.join(groups[suit]) + suit
    return result


@app.route('/predict', methods=['POST'])
def predict():
    if model is None:
        return jsonify({"error": "model not loaded"}), 503

    if 'image' not in request.files:
        return jsonify({"error": "no image field"}), 400

    file = request.files['image']
    image_bytes = file.read()
    image = Image.open(io.BytesIO(image_bytes))

    results = model.predict(image, conf=CONFIDENCE_THRESHOLD, verbose=False)

    detections = []
    for result in results:
        boxes = result.boxes
        for i in range(len(boxes)):
            cls_id = int(boxes.cls[i])
            conf = float(boxes.conf[i])
            x1, y1, x2, y2 = boxes.xyxy[i].tolist()

            if cls_id >= len(CLASS_NAMES):
                continue
            class_name = CLASS_NAMES[cls_id]
            if class_name == 'UNKNOWN':
                continue

            tile_name = RED_FIVE_MAP.get(class_name, class_name)
            detections.append({
                "tile": tile_name,
                "confidence": round(conf, 3),
                "x": (x1 + x2) / 2,
                "bbox": [x1, y1, x2, y2]
            })

    detections.sort(key=lambda d: d["x"])
    tiles = [d["tile"] for d in detections]
    tile_string = build_tile_string(tiles)

    return jsonify({
        "tiles": tile_string,
        "count": len(tiles),
        "detections": [{"tile": d["tile"], "confidence": d["confidence"]} for d in detections]
    })


@app.route('/health', methods=['GET'])
def health():
    return jsonify({"status": "ok", "model_loaded": model is not None})


if __name__ == '__main__':
    if load_model():
        print(f"Model loaded: {MODEL_PATH}")
    else:
        print(f"WARNING: Model not found at {MODEL_PATH}, /predict will return 503")
    app.run(host='0.0.0.0', port=5000)
