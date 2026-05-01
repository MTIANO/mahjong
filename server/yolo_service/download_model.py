"""
Download YOLOv11s mahjong tile detection model from GitHub.
Source: https://github.com/nikmomo/Mahjong-YOLO
"""
import os
import urllib.request

MODEL_URL = "https://raw.githubusercontent.com/nikmomo/Mahjong-YOLO/main/trained_models_v2/yolo11s_best.pt"
MODEL_DIR = "models"
MODEL_PATH = os.path.join(MODEL_DIR, "yolo11s_best.pt")


def download():
    os.makedirs(MODEL_DIR, exist_ok=True)
    if os.path.exists(MODEL_PATH):
        print(f"Model already exists: {MODEL_PATH}")
        return

    print(f"Downloading model from {MODEL_URL} ...")
    urllib.request.urlretrieve(MODEL_URL, MODEL_PATH)
    size_mb = os.path.getsize(MODEL_PATH) / (1024 * 1024)
    print(f"Done! Saved to {MODEL_PATH} ({size_mb:.1f} MB)")


if __name__ == '__main__':
    download()
