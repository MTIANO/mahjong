import os
import io
import numpy as np
from flask import Flask, request, jsonify
from PIL import Image
import onnxruntime as ort

app = Flask(__name__)

MODEL_PATH = os.environ.get("YOLO_MODEL_PATH", "models/yolo11s_best.onnx")
CONFIDENCE_THRESHOLD = float(os.environ.get("YOLO_CONFIDENCE", "0.5"))
INPUT_SIZE = 640

CLASS_NAMES = [
    '1m', '1p', '1s', '1z', '2m', '2p', '2s', '2z',
    '3m', '3p', '3s', '3z', '4m', '4p', '4s', '4z',
    '5m', '5p', '5s', '5z', '6m', '6p', '6s', '6z',
    '7m', '7p', '7s', '7z', '8m', '8p', '8s',
    '9m', '9p', '9s', 'UNKNOWN', '0m', '0p', '0s'
]

RED_FIVE_MAP = {'0m': '5m', '0p': '5p', '0s': '5s'}

session = None


def load_model():
    global session
    if not os.path.exists(MODEL_PATH):
        return False
    session = ort.InferenceSession(MODEL_PATH, providers=['CPUExecutionProvider'])
    return True


def preprocess(image):
    img = image.convert('RGB')
    orig_w, orig_h = img.size

    scale = min(INPUT_SIZE / orig_w, INPUT_SIZE / orig_h)
    new_w, new_h = int(orig_w * scale), int(orig_h * scale)
    img = img.resize((new_w, new_h), Image.BILINEAR)

    padded = Image.new('RGB', (INPUT_SIZE, INPUT_SIZE), (114, 114, 114))
    pad_x = (INPUT_SIZE - new_w) // 2
    pad_y = (INPUT_SIZE - new_h) // 2
    padded.paste(img, (pad_x, pad_y))

    data = np.array(padded, dtype=np.float32) / 255.0
    data = data.transpose(2, 0, 1)
    data = np.expand_dims(data, axis=0)

    return data, scale, pad_x, pad_y


def postprocess(output, scale, pad_x, pad_y, conf_threshold):
    predictions = output[0]

    if predictions.shape[-1] == 4 + len(CLASS_NAMES):
        preds = predictions[0]
    elif predictions.shape[1] == 4 + len(CLASS_NAMES):
        preds = predictions[0]
    else:
        preds = predictions[0].T

    detections = []
    for pred in preds:
        if len(pred) < 4 + len(CLASS_NAMES):
            continue

        box = pred[:4]
        class_scores = pred[4:]

        max_score = np.max(class_scores)
        if max_score < conf_threshold:
            continue

        cls_id = int(np.argmax(class_scores))

        cx, cy, w, h = box
        x1 = (cx - w / 2 - pad_x) / scale
        y1 = (cy - h / 2 - pad_y) / scale
        x2 = (cx + w / 2 - pad_x) / scale
        y2 = (cy + h / 2 - pad_y) / scale

        if cls_id >= len(CLASS_NAMES):
            continue
        class_name = CLASS_NAMES[cls_id]
        if class_name == 'UNKNOWN':
            continue

        tile_name = RED_FIVE_MAP.get(class_name, class_name)
        detections.append({
            "tile": tile_name,
            "confidence": float(max_score),
            "x": float((x1 + x2) / 2),
            "bbox": [float(x1), float(y1), float(x2), float(y2)]
        })

    # NMS
    detections = nms(detections, iou_threshold=0.5)
    detections.sort(key=lambda d: d["x"])
    return detections


def nms(detections, iou_threshold=0.5):
    if not detections:
        return []

    detections.sort(key=lambda d: d["confidence"], reverse=True)
    keep = []

    while detections:
        best = detections.pop(0)
        keep.append(best)
        detections = [d for d in detections if iou(best["bbox"], d["bbox"]) < iou_threshold]

    return keep


def iou(box1, box2):
    x1 = max(box1[0], box2[0])
    y1 = max(box1[1], box2[1])
    x2 = min(box1[2], box2[2])
    y2 = min(box1[3], box2[3])

    inter = max(0, x2 - x1) * max(0, y2 - y1)
    area1 = (box1[2] - box1[0]) * (box1[3] - box1[1])
    area2 = (box2[2] - box2[0]) * (box2[3] - box2[1])
    union = area1 + area2 - inter

    return inter / union if union > 0 else 0


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
    if session is None:
        return jsonify({"error": "model not loaded"}), 503

    if 'image' not in request.files:
        return jsonify({"error": "no image field"}), 400

    file = request.files['image']
    image_bytes = file.read()
    image = Image.open(io.BytesIO(image_bytes))

    input_data, scale, pad_x, pad_y = preprocess(image)

    input_name = session.get_inputs()[0].name
    output = session.run(None, {input_name: input_data})

    detections = postprocess(output, scale, pad_x, pad_y, CONFIDENCE_THRESHOLD)
    tiles = [d["tile"] for d in detections]
    tile_string = build_tile_string(tiles)

    return jsonify({
        "tiles": tile_string,
        "count": len(tiles),
        "detections": [{"tile": d["tile"], "confidence": round(d["confidence"], 3)} for d in detections]
    })


@app.route('/health', methods=['GET'])
def health():
    return jsonify({"status": "ok", "model_loaded": session is not None})


if __name__ == '__main__':
    if load_model():
        print(f"Model loaded: {MODEL_PATH}")
    else:
        print(f"WARNING: Model not found at {MODEL_PATH}")
        print("Run convert_model.py first to convert .pt to .onnx")
    app.run(host='0.0.0.0', port=5000)
