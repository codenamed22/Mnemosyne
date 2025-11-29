// Gallery App JavaScript

const csrfToken = document.getElementById('csrfToken')?.value || '';
const currentUserID = parseInt(document.getElementById('currentUserID')?.value || '0');
const isAdmin = document.getElementById('isAdmin')?.value === 'true';

let currentTab = 'my-photos';
let currentPhotos = [];
let currentPhotoIndex = -1;
let zoomLevel = 1;
let isDragging = false;
let dragStart = { x: 0, y: 0 };
let imageOffset = { x: 0, y: 0 };

// Initialize app
document.addEventListener('DOMContentLoaded', () => {
    setupTabHandlers();
    setupUploadHandlers();
    setupViewerHandlers();
    loadPhotos();
});

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

// Setup tab handlers
function setupTabHandlers() {
    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.addEventListener('click', () => {
            document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
            currentTab = btn.dataset.tab;
            loadPhotos();
        });
    });
}

// Load photos based on current tab
async function loadPhotos() {
    const gallery = document.getElementById('gallery');
    gallery.innerHTML = '<div class="loading">Loading photos...</div>';

    let endpoint = '/api/photos/my';
    if (currentTab === 'family') {
        endpoint = '/api/photos/shared';
    } else if (currentTab === 'all' && isAdmin) {
        endpoint = '/api/photos/all';
    }

    try {
        const response = await fetch(endpoint);

        if (!response.ok) {
            if (response.status === 401) {
                window.location.href = '/login';
                return;
            }
            throw new Error('Failed to load photos');
        }

        currentPhotos = await response.json() || [];
        displayPhotos(currentPhotos);
    } catch (error) {
        console.error('Error loading photos:', error);
        gallery.innerHTML = '<div class="error">Failed to load photos</div>';
    }
}

// Display photos in gallery
function displayPhotos(photos) {
    const gallery = document.getElementById('gallery');
    const emptyState = document.getElementById('emptyState');
    const emptyTitle = document.getElementById('emptyTitle');
    const emptyMessage = document.getElementById('emptyMessage');

    if (!photos || photos.length === 0) {
        gallery.style.display = 'none';
        emptyState.style.display = 'block';

        if (currentTab === 'my-photos') {
            emptyTitle.textContent = 'No Photos Yet';
            emptyMessage.textContent = 'Upload your first photo to get started!';
        } else if (currentTab === 'family') {
            emptyTitle.textContent = 'Family Area Empty';
            emptyMessage.textContent = 'No photos have been shared to the family area yet.';
        } else {
            emptyTitle.textContent = 'No Photos';
            emptyMessage.textContent = 'No photos in the system yet.';
        }
        return;
    }

    gallery.style.display = 'grid';
    emptyState.style.display = 'none';

    // Sort by upload date (newest first)
    photos.sort((a, b) => new Date(b.uploaded_at) - new Date(a.uploaded_at));

    gallery.innerHTML = photos.map((photo, index) => {
        const safeFilename = escapeAttr(photo.filename);
        const displayFilename = escapeHtml(photo.filename);
        const ownerBadge = (currentTab !== 'my-photos' && photo.username) 
            ? `<span class="photo-badge">${escapeHtml(photo.username)}</span>` 
            : '';
        const sharedBadge = photo.is_shared ? '<span class="photo-badge shared">Shared</span>' : '';

        return `
            <div class="photo-card" data-index="${index}" onclick="openViewer(${index})">
                <img src="${escapeAttr(photo.thumbnail_url)}" alt="${safeFilename}" loading="lazy">
                <div class="photo-info">
                    <div class="photo-filename">${displayFilename}</div>
                    <div class="photo-meta">
                        ${formatFileSize(photo.size)} • ${formatDate(photo.uploaded_at)}
                        ${ownerBadge}${sharedBadge}
                    </div>
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

    uploadBtn?.addEventListener('click', () => {
        uploadArea.style.display = 'flex';
    });

    closeUpload?.addEventListener('click', () => {
        uploadArea.style.display = 'none';
    });

    uploadBox?.addEventListener('click', (e) => {
        if (e.target === closeUpload) return;
        fileInput?.click();
    });

    fileInput?.addEventListener('change', (e) => {
        handleFiles(e.target.files);
    });

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

    document.getElementById('uploadArea').style.display = 'none';
    uploadBox.style.display = 'block';
    uploadProgress.style.display = 'none';
    progressFill.style.width = '0%';
    document.getElementById('fileInput').value = '';

    // Switch to my photos tab and reload
    document.querySelector('[data-tab="my-photos"]').click();
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

// ==================== PHOTO VIEWER ====================

function setupViewerHandlers() {
    const viewer = document.getElementById('photoViewer');
    const viewerImage = document.getElementById('viewerImage');
    
    // Close button and overlay
    document.getElementById('viewerClose')?.addEventListener('click', closeViewer);
    document.querySelector('.viewer-overlay')?.addEventListener('click', closeViewer);
    
    // Navigation
    document.getElementById('viewerPrev')?.addEventListener('click', () => navigatePhoto(-1));
    document.getElementById('viewerNext')?.addEventListener('click', () => navigatePhoto(1));
    
    // Zoom controls
    document.getElementById('viewerZoomIn')?.addEventListener('click', () => zoom(0.25));
    document.getElementById('viewerZoomOut')?.addEventListener('click', () => zoom(-0.25));
    document.getElementById('viewerZoomReset')?.addEventListener('click', resetZoom);
    
    // Actions
    document.getElementById('viewerShare')?.addEventListener('click', toggleShareCurrentPhoto);
    document.getElementById('viewerDelete')?.addEventListener('click', deleteCurrentPhoto);
    
    // Keyboard navigation
    document.addEventListener('keydown', handleViewerKeyboard);
    
    // Mouse wheel zoom
    viewerImage?.addEventListener('wheel', (e) => {
        if (viewer.style.display === 'none') return;
        e.preventDefault();
        zoom(e.deltaY > 0 ? -0.1 : 0.1);
    });
    
    // Image drag for panning when zoomed
    viewerImage?.addEventListener('mousedown', startDrag);
    document.addEventListener('mousemove', drag);
    document.addEventListener('mouseup', endDrag);
    
    // Touch support
    let touchStartX = 0;
    let touchStartY = 0;
    let touchStartDist = 0;
    
    viewerImage?.addEventListener('touchstart', (e) => {
        if (e.touches.length === 1) {
            touchStartX = e.touches[0].clientX;
            touchStartY = e.touches[0].clientY;
        } else if (e.touches.length === 2) {
            // Pinch to zoom
            touchStartDist = Math.hypot(
                e.touches[0].clientX - e.touches[1].clientX,
                e.touches[0].clientY - e.touches[1].clientY
            );
        }
    });
    
    viewerImage?.addEventListener('touchmove', (e) => {
        if (e.touches.length === 2 && touchStartDist > 0) {
            e.preventDefault();
            const currentDist = Math.hypot(
                e.touches[0].clientX - e.touches[1].clientX,
                e.touches[0].clientY - e.touches[1].clientY
            );
            const delta = (currentDist - touchStartDist) / 200;
            zoom(delta);
            touchStartDist = currentDist;
        }
    });
    
    viewerImage?.addEventListener('touchend', (e) => {
        if (e.changedTouches.length === 1 && zoomLevel <= 1) {
            const touchEndX = e.changedTouches[0].clientX;
            const touchEndY = e.changedTouches[0].clientY;
            const diffX = touchEndX - touchStartX;
            const diffY = touchEndY - touchStartY;
            
            // Swipe detection
            if (Math.abs(diffX) > 50 && Math.abs(diffY) < 100) {
                if (diffX > 0) {
                    navigatePhoto(-1); // Swipe right = previous
                } else {
                    navigatePhoto(1); // Swipe left = next
                }
            }
        }
        touchStartDist = 0;
    });
    
    // Image load handler
    viewerImage?.addEventListener('load', () => {
        document.getElementById('viewerLoading').style.display = 'none';
        viewerImage.style.opacity = '1';
    });
}

function handleViewerKeyboard(e) {
    const viewer = document.getElementById('photoViewer');
    if (viewer.style.display === 'none') return;
    
    switch (e.key) {
        case 'Escape':
            closeViewer();
            break;
        case 'ArrowLeft':
            navigatePhoto(-1);
            break;
        case 'ArrowRight':
            navigatePhoto(1);
            break;
        case '+':
        case '=':
            zoom(0.25);
            break;
        case '-':
            zoom(-0.25);
            break;
        case '0':
            resetZoom();
            break;
    }
}

function openViewer(index) {
    if (index < 0 || index >= currentPhotos.length) return;
    
    currentPhotoIndex = index;
    const photo = currentPhotos[index];
    
    const viewer = document.getElementById('photoViewer');
    const viewerImage = document.getElementById('viewerImage');
    const viewerLoading = document.getElementById('viewerLoading');
    
    // Reset zoom
    resetZoom();
    
    // Show loading
    viewerLoading.style.display = 'flex';
    viewerImage.style.opacity = '0';
    
    // Set image source
    viewerImage.src = photo.original_url;
    
    // Update info
    document.getElementById('viewerFilename').textContent = photo.filename;
    
    const ownerEl = document.getElementById('viewerOwner');
    if (photo.username && photo.user_id !== currentUserID) {
        ownerEl.textContent = `by ${photo.username}`;
        ownerEl.style.display = 'inline';
    } else {
        ownerEl.style.display = 'none';
    }
    
    // Update counter
    document.getElementById('viewerCounter').textContent = `${index + 1} / ${currentPhotos.length}`;
    
    // Update download link
    const downloadBtn = document.getElementById('viewerDownload');
    downloadBtn.href = photo.original_url;
    downloadBtn.download = photo.filename;
    
    // Show/hide share button
    const shareBtn = document.getElementById('viewerShare');
    if (photo.user_id === currentUserID) {
        shareBtn.style.display = 'flex';
        shareBtn.querySelector('.action-text').textContent = photo.is_shared ? 'Unshare' : 'Share';
    } else {
        shareBtn.style.display = 'none';
    }
    
    // Show/hide delete button
    const deleteBtn = document.getElementById('viewerDelete');
    if (photo.user_id === currentUserID || isAdmin) {
        deleteBtn.style.display = 'flex';
    } else {
        deleteBtn.style.display = 'none';
    }
    
    // Update navigation buttons
    document.getElementById('viewerPrev').style.visibility = index > 0 ? 'visible' : 'hidden';
    document.getElementById('viewerNext').style.visibility = index < currentPhotos.length - 1 ? 'visible' : 'hidden';
    
    // Show viewer
    viewer.style.display = 'flex';
    document.body.style.overflow = 'hidden';
}

function closeViewer() {
    const viewer = document.getElementById('photoViewer');
    viewer.style.display = 'none';
    document.body.style.overflow = '';
    currentPhotoIndex = -1;
    resetZoom();
}

function navigatePhoto(direction) {
    const newIndex = currentPhotoIndex + direction;
    if (newIndex >= 0 && newIndex < currentPhotos.length) {
        openViewer(newIndex);
    }
}

function zoom(delta) {
    zoomLevel = Math.max(0.5, Math.min(5, zoomLevel + delta));
    applyZoom();
}

function resetZoom() {
    zoomLevel = 1;
    imageOffset = { x: 0, y: 0 };
    applyZoom();
}

function applyZoom() {
    const viewerImage = document.getElementById('viewerImage');
    viewerImage.style.transform = `scale(${zoomLevel}) translate(${imageOffset.x}px, ${imageOffset.y}px)`;
    viewerImage.style.cursor = zoomLevel > 1 ? 'grab' : 'default';
}

function startDrag(e) {
    if (zoomLevel <= 1) return;
    isDragging = true;
    dragStart = { x: e.clientX - imageOffset.x, y: e.clientY - imageOffset.y };
    document.getElementById('viewerImage').style.cursor = 'grabbing';
}

function drag(e) {
    if (!isDragging) return;
    e.preventDefault();
    imageOffset = {
        x: e.clientX - dragStart.x,
        y: e.clientY - dragStart.y
    };
    applyZoom();
}

function endDrag() {
    isDragging = false;
    if (zoomLevel > 1) {
        document.getElementById('viewerImage').style.cursor = 'grab';
    }
}

async function toggleShareCurrentPhoto() {
    if (currentPhotoIndex < 0) return;
    const photo = currentPhotos[currentPhotoIndex];
    
    try {
        const response = await fetch(`/api/photos/${photo.id}/share`, {
            method: 'POST',
            headers: { 'X-CSRF-Token': csrfToken }
        });
        
        if (!response.ok) throw new Error('Failed to update sharing');
        
        const result = await response.json();
        photo.is_shared = result.is_shared;
        
        // Update button text
        document.getElementById('viewerShare').querySelector('.action-text').textContent = 
            result.is_shared ? 'Unshare' : 'Share';
        
        // Refresh gallery
        displayPhotos(currentPhotos);
        
    } catch (error) {
        console.error('Error sharing photo:', error);
        showError('Failed to update sharing');
    }
}

async function deleteCurrentPhoto() {
    if (currentPhotoIndex < 0) return;
    const photo = currentPhotos[currentPhotoIndex];
    
    if (!confirm(`Are you sure you want to delete "${photo.filename}"?`)) {
        return;
    }
    
    try {
        const response = await fetch(`/api/photos/${photo.id}`, {
            method: 'DELETE',
            headers: { 'X-CSRF-Token': csrfToken }
        });
        
        if (!response.ok) throw new Error('Failed to delete photo');
        
        // Remove from array
        currentPhotos.splice(currentPhotoIndex, 1);
        
        if (currentPhotos.length === 0) {
            closeViewer();
            displayPhotos(currentPhotos);
        } else if (currentPhotoIndex >= currentPhotos.length) {
            openViewer(currentPhotos.length - 1);
            displayPhotos(currentPhotos);
        } else {
            openViewer(currentPhotoIndex);
            displayPhotos(currentPhotos);
        }
        
    } catch (error) {
        console.error('Error deleting photo:', error);
        showError('Failed to delete photo');
    }
}

// ==================== UTILITIES ====================

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
    alert(message);
}
