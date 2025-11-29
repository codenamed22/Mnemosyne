// Gallery App JavaScript

const csrfToken = document.getElementById('csrfToken')?.value || '';
let currentPhotos = [];

// Initialize app
document.addEventListener('DOMContentLoaded', () => {
    loadPhotos();
    setupUploadHandlers();
    setupModalHandlers();
});

// Load and display photos
async function loadPhotos() {
    try {
        const response = await fetch('/api/photos/list');
        
        if (!response.ok) {
            if (response.status === 401) {
                window.location.href = '/login';
                return;
            }
            throw new Error('Failed to load photos');
        }
        
        currentPhotos = await response.json();
        displayPhotos(currentPhotos);
    } catch (error) {
        console.error('Error loading photos:', error);
        showError('Failed to load photos');
    }
}

// Escape HTML to prevent XSS
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Escape for use in HTML attributes
function escapeAttr(text) {
    return text
        .replace(/&/g, '&amp;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;');
}

// Display photos in gallery
function displayPhotos(photos) {
    const gallery = document.getElementById('gallery');
    const emptyState = document.getElementById('emptyState');
    
    if (!photos || photos.length === 0) {
        gallery.style.display = 'none';
        emptyState.style.display = 'block';
        return;
    }
    
    gallery.style.display = 'grid';
    emptyState.style.display = 'none';
    
    // Sort by upload date (newest first)
    photos.sort((a, b) => new Date(b.uploaded_at) - new Date(a.uploaded_at));
    
    gallery.innerHTML = photos.map(photo => {
        const safeFilename = escapeAttr(photo.filename);
        const displayFilename = escapeHtml(photo.filename);
        return `
            <div class="photo-card" data-filename="${safeFilename}" onclick="openPhotoModal(this.dataset.filename)">
                <img src="${escapeAttr(photo.thumbnail_url)}" alt="${safeFilename}" loading="lazy">
                <div class="photo-info">
                    <div class="photo-filename">${displayFilename}</div>
                    <div class="photo-meta">${formatFileSize(photo.size)} • ${formatDate(photo.uploaded_at)}</div>
                </div>
            </div>
        `;
    }).join('');
}

// Setup upload handlers
function setupUploadHandlers() {
    const uploadBtn = document.getElementById('uploadBtn');
    const uploadArea = document.getElementById('uploadArea');
    const uploadBox = document.querySelector('.upload-box');
    const closeUpload = document.getElementById('closeUpload');
    const fileInput = document.getElementById('fileInput');
    
    // Show upload area
    uploadBtn?.addEventListener('click', () => {
        uploadArea.style.display = 'flex';
    });
    
    // Close upload area
    closeUpload?.addEventListener('click', () => {
        uploadArea.style.display = 'none';
    });
    
    // Click to browse
    uploadBox?.addEventListener('click', (e) => {
        if (e.target === closeUpload) return;
        fileInput?.click();
    });
    
    // File input change
    fileInput?.addEventListener('change', (e) => {
        handleFiles(e.target.files);
    });
    
    // Drag and drop
    uploadBox?.addEventListener('dragover', (e) => {
        e.preventDefault();
        uploadBox.classList.add('drag-over');
    });
    
    uploadBox?.addEventListener('dragleave', () => {
        uploadBox.classList.remove('drag-over');
    });
    
    uploadBox?.addEventListener('drop', (e) => {
        e.preventDefault();
        uploadBox.classList.remove('drag-over');
        handleFiles(e.dataTransfer.files);
    });
}

// Handle file uploads
async function handleFiles(files) {
    if (!files || files.length === 0) return;
    
    const uploadBox = document.querySelector('.upload-box');
    const uploadProgress = document.getElementById('uploadProgress');
    const progressFill = document.getElementById('progressFill');
    const progressText = document.getElementById('progressText');
    
    uploadBox.style.display = 'none';
    uploadProgress.style.display = 'block';
    
    let completed = 0;
    const total = files.length;
    
    for (let i = 0; i < files.length; i++) {
        const file = files[i];
        
        // Validate file type
        if (!file.type.startsWith('image/')) {
            console.warn('Skipping non-image file:', file.name);
            completed++;
            continue;
        }
        
        try {
            await uploadFile(file);
            completed++;
            
            const progress = (completed / total) * 100;
            progressFill.style.width = progress + '%';
            progressText.textContent = `Uploading ${completed}/${total}...`;
        } catch (error) {
            console.error('Error uploading file:', file.name, error);
            showError(`Failed to upload ${file.name}`);
        }
    }
    
    // Hide upload area and reload photos
    document.getElementById('uploadArea').style.display = 'none';
    uploadBox.style.display = 'block';
    uploadProgress.style.display = 'none';
    progressFill.style.width = '0%';
    
    // Clear file input
    document.getElementById('fileInput').value = '';
    
    // Reload gallery
    loadPhotos();
}

// Upload a single file
async function uploadFile(file) {
    const formData = new FormData();
    formData.append('photo', file);
    formData.append('csrf_token', csrfToken);
    
    const response = await fetch('/api/photos/upload', {
        method: 'POST',
        headers: {
            'X-CSRF-Token': csrfToken
        },
        body: formData
    });
    
    if (!response.ok) {
        throw new Error(`Upload failed: ${response.statusText}`);
    }
    
    return await response.json();
}

// Setup modal handlers
function setupModalHandlers() {
    const modal = document.getElementById('photoModal');
    const modalOverlay = document.querySelector('.modal-overlay');
    const modalClose = document.querySelector('.modal-close');
    const deleteBtn = document.getElementById('deleteBtn');
    
    modalOverlay?.addEventListener('click', closePhotoModal);
    modalClose?.addEventListener('click', closePhotoModal);
    deleteBtn?.addEventListener('click', deleteCurrentPhoto);
    
    // Close on Escape key
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape' && modal.style.display !== 'none') {
            closePhotoModal();
        }
    });
}

// Open photo modal
function openPhotoModal(filename) {
    const modal = document.getElementById('photoModal');
    const modalImage = document.getElementById('modalImage');
    const downloadBtn = document.getElementById('downloadBtn');
    
    modalImage.src = `/api/photos/original/${filename}`;
    downloadBtn.href = `/api/photos/original/${filename}`;
    downloadBtn.download = filename;
    
    modal.style.display = 'block';
    modal.dataset.currentFilename = filename;
}

// Close photo modal
function closePhotoModal() {
    const modal = document.getElementById('photoModal');
    modal.style.display = 'none';
    modal.dataset.currentFilename = '';
}

// Delete current photo
async function deleteCurrentPhoto() {
    const modal = document.getElementById('photoModal');
    const filename = modal.dataset.currentFilename;
    
    if (!filename) return;
    
    if (!confirm(`Are you sure you want to delete ${filename}?`)) {
        return;
    }
    
    try {
        const response = await fetch(`/api/photos/${filename}`, {
            method: 'DELETE',
            headers: {
                'X-CSRF-Token': csrfToken
            }
        });
        
        if (!response.ok) {
            throw new Error('Failed to delete photo');
        }
        
        closePhotoModal();
        loadPhotos();
    } catch (error) {
        console.error('Error deleting photo:', error);
        showError('Failed to delete photo');
    }
}

// Utility functions
function formatFileSize(bytes) {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
}

function formatDate(dateString) {
    const date = new Date(dateString);
    const now = new Date();
    const diffMs = now - date;
    const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));
    
    if (diffDays === 0) return 'Today';
    if (diffDays === 1) return 'Yesterday';
    if (diffDays < 7) return `${diffDays} days ago`;
    
    return date.toLocaleDateString();
}

function showError(message) {
    // Simple error display - could be enhanced with a toast notification
    alert(message);
}

