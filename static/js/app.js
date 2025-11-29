// Gallery App JavaScript

const csrfToken = document.getElementById('csrfToken')?.value || '';
const currentUserID = parseInt(document.getElementById('currentUserID')?.value || '0');
const isAdmin = document.getElementById('isAdmin')?.value === 'true';

let currentTab = 'my-photos';
let currentPhotos = [];
let currentPhoto = null;

// Initialize app
document.addEventListener('DOMContentLoaded', () => {
    setupTabHandlers();
    setupUploadHandlers();
    setupModalHandlers();
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

    gallery.innerHTML = photos.map(photo => {
        const safeFilename = escapeAttr(photo.filename);
        const displayFilename = escapeHtml(photo.filename);
        const ownerBadge = (currentTab !== 'my-photos' && photo.username) 
            ? `<span class="photo-badge">${escapeHtml(photo.username)}</span>` 
            : '';
        const sharedBadge = photo.is_shared ? '<span class="photo-badge shared">Shared</span>' : '';

        return `
            <div class="photo-card" data-photo-id="${photo.id}" onclick="openPhotoModal(${photo.id})">
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

// Setup modal handlers
function setupModalHandlers() {
    const modal = document.getElementById('photoModal');
    const modalOverlay = document.querySelector('.modal-overlay');
    const modalClose = document.querySelector('.modal-close');
    const deleteBtn = document.getElementById('deleteBtn');
    const shareBtn = document.getElementById('shareBtn');

    modalOverlay?.addEventListener('click', closePhotoModal);
    modalClose?.addEventListener('click', closePhotoModal);
    deleteBtn?.addEventListener('click', deleteCurrentPhoto);
    shareBtn?.addEventListener('click', toggleSharePhoto);

    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape' && modal.style.display !== 'none') {
            closePhotoModal();
        }
    });
}

// Open photo modal
function openPhotoModal(photoId) {
    const photo = currentPhotos.find(p => p.id === photoId);
    if (!photo) return;

    currentPhoto = photo;

    const modal = document.getElementById('photoModal');
    const modalImage = document.getElementById('modalImage');
    const downloadBtn = document.getElementById('downloadBtn');
    const deleteBtn = document.getElementById('deleteBtn');
    const shareBtn = document.getElementById('shareBtn');
    const modalFilename = document.getElementById('modalFilename');
    const modalOwner = document.getElementById('modalOwner');

    modalImage.src = photo.original_url;
    downloadBtn.href = photo.original_url;
    downloadBtn.download = photo.filename;
    modalFilename.textContent = photo.filename;

    // Show owner if not own photo
    if (photo.username && photo.user_id !== currentUserID) {
        modalOwner.textContent = `by ${photo.username}`;
        modalOwner.style.display = 'inline';
    } else {
        modalOwner.style.display = 'none';
    }

    // Show/hide share button (only for own photos)
    if (photo.user_id === currentUserID) {
        shareBtn.style.display = 'inline-block';
        shareBtn.textContent = photo.is_shared ? 'Unshare from Family' : 'Share to Family';
    } else {
        shareBtn.style.display = 'none';
    }

    // Show/hide delete button (owner or admin)
    if (photo.user_id === currentUserID || isAdmin) {
        deleteBtn.style.display = 'inline-block';
    } else {
        deleteBtn.style.display = 'none';
    }

    modal.style.display = 'block';
}

// Close photo modal
function closePhotoModal() {
    const modal = document.getElementById('photoModal');
    modal.style.display = 'none';
    currentPhoto = null;
}

// Toggle share status
async function toggleSharePhoto() {
    if (!currentPhoto) return;

    try {
        const response = await fetch(`/api/photos/${currentPhoto.id}/share`, {
            method: 'POST',
            headers: {
                'X-CSRF-Token': csrfToken
            }
        });

        if (!response.ok) {
            throw new Error('Failed to update sharing');
        }

        const result = await response.json();
        currentPhoto.is_shared = result.is_shared;

        const shareBtn = document.getElementById('shareBtn');
        shareBtn.textContent = result.is_shared ? 'Unshare from Family' : 'Share to Family';

        // Update the photo in the list
        const photoInList = currentPhotos.find(p => p.id === currentPhoto.id);
        if (photoInList) {
            photoInList.is_shared = result.is_shared;
        }

        // Refresh display
        displayPhotos(currentPhotos);

    } catch (error) {
        console.error('Error sharing photo:', error);
        showError('Failed to update sharing');
    }
}

// Delete current photo
async function deleteCurrentPhoto() {
    if (!currentPhoto) return;

    if (!confirm(`Are you sure you want to delete ${currentPhoto.filename}?`)) {
        return;
    }

    try {
        const response = await fetch(`/api/photos/${currentPhoto.id}`, {
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
    alert(message);
}
