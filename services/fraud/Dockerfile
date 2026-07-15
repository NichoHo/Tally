FROM python:3.11-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
# Train the IsolationForest at build time so the image ships with a reproducible
# model (synthetic data, fixed seed).
RUN python train.py
CMD ["python", "consumer.py"]
