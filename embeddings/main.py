"""
CLIP Embedding Service for Mnemosyne
Generates 512-dimensional embeddings for images using CLIP ViT-B/32
"""

import io
import base64
import logging
from typing import List, Optional
from contextlib import asynccontextmanager

import torch
from PIL import Image
from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel
from transformers import CLIPProcessor, CLIPModel

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Global model references
model: Optional[CLIPModel] = None
processor: Optional[CLIPProcessor] = None
device: str = "cpu"


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Load CLIP model on startup"""
    global model, processor, device
    
    logger.info("Loading CLIP model (ViT-B/32)...")
    
    # Use GPU if available
    device = "cuda" if torch.cuda.is_available() else "cpu"
    logger.info(f"Using device: {device}")
    
    # Load model and processor
    model_name = "openai/clip-vit-base-patch32"
    model = CLIPModel.from_pretrained(model_name).to(device)
    processor = CLIPProcessor.from_pretrained(model_name)
    
    # Set to evaluation mode
    model.eval()
    
    logger.info("CLIP model loaded successfully!")
    
    yield
    
    # Cleanup
    logger.info("Shutting down CLIP service...")


app = FastAPI(
    title="Mnemosyne CLIP Service",
    description="Generate image embeddings using CLIP ViT-B/32",
    version="1.0.0",
    lifespan=lifespan
)

# Allow CORS for local development
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


class EmbeddingRequest(BaseModel):
    """Request to generate embedding for a single image"""
    image_base64: str  # Base64-encoded image data
    image_id: Optional[str] = None  # Optional ID for tracking


class EmbeddingResponse(BaseModel):
    """Response containing the embedding vector"""
    image_id: Optional[str]
    embedding: List[float]  # 512-dimensional vector
    dimension: int


class BatchEmbeddingRequest(BaseModel):
    """Request to generate embeddings for multiple images"""
    images: List[EmbeddingRequest]


class BatchEmbeddingResponse(BaseModel):
    """Response containing multiple embeddings"""
    embeddings: List[EmbeddingResponse]


class HealthResponse(BaseModel):
    """Health check response"""
    status: str
    model_loaded: bool
    device: str


def decode_image(image_base64: str) -> Image.Image:
    """Decode base64 image to PIL Image"""
    try:
        # Handle data URL format (data:image/jpeg;base64,...)
        if "," in image_base64:
            image_base64 = image_base64.split(",")[1]
        
        image_data = base64.b64decode(image_base64)
        image = Image.open(io.BytesIO(image_data)).convert("RGB")
        return image
    except Exception as e:
        raise ValueError(f"Failed to decode image: {e}")


def generate_embedding(image: Image.Image) -> List[float]:
    """Generate CLIP embedding for an image"""
    global model, processor, device
    
    if model is None or processor is None:
        raise RuntimeError("Model not loaded")
    
    # Process image
    inputs = processor(images=image, return_tensors="pt").to(device)
    
    # Generate embedding
    with torch.no_grad():
        image_features = model.get_image_features(**inputs)
        
        # Normalize the embedding
        image_features = image_features / image_features.norm(p=2, dim=-1, keepdim=True)
        
        # Convert to list
        embedding = image_features.cpu().numpy().flatten().tolist()
    
    return embedding


@app.get("/health", response_model=HealthResponse)
async def health_check():
    """Check service health and model status"""
    return HealthResponse(
        status="healthy",
        model_loaded=model is not None,
        device=device
    )


@app.post("/embed", response_model=EmbeddingResponse)
async def create_embedding(request: EmbeddingRequest):
    """Generate embedding for a single image"""
    try:
        # Decode image
        image = decode_image(request.image_base64)
        
        # Generate embedding
        embedding = generate_embedding(image)
        
        return EmbeddingResponse(
            image_id=request.image_id,
            embedding=embedding,
            dimension=len(embedding)
        )
    
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        logger.error(f"Error generating embedding: {e}")
        raise HTTPException(status_code=500, detail=f"Embedding generation failed: {e}")


@app.post("/embed/batch", response_model=BatchEmbeddingResponse)
async def create_batch_embeddings(request: BatchEmbeddingRequest):
    """Generate embeddings for multiple images"""
    results = []
    
    for img_request in request.images:
        try:
            image = decode_image(img_request.image_base64)
            embedding = generate_embedding(image)
            
            results.append(EmbeddingResponse(
                image_id=img_request.image_id,
                embedding=embedding,
                dimension=len(embedding)
            ))
        except Exception as e:
            logger.error(f"Error processing image {img_request.image_id}: {e}")
            # Continue processing other images
            continue
    
    return BatchEmbeddingResponse(embeddings=results)


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="127.0.0.1", port=8081)

