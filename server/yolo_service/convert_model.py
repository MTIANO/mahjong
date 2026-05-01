"""
Convert YOLOv11 .pt model to ONNX format.
Run this on your local machine (needs ultralytics + torch), then upload the .onnx to server.

Usage:
    pip install ultralytics
    python convert_model.py
"""
from ultralytics import YOLO

MODEL_PT = "models/yolo11s_best.pt"
MODEL_ONNX = "models/yolo11s_best.onnx"

model = YOLO(MODEL_PT)
model.export(format="onnx", imgsz=640, simplify=True)
print(f"Exported to {MODEL_ONNX}")
