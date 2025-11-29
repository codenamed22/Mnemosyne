// Mnemosyne Gallery

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

// Selection state
let selectMode = false;
let selectedPhotos = new Set();

// Device detection
const isMobile = /iPhone|iPad|iPod|Android/i.test(navigator.userAgent);
const isIOS = /iPhone|iPad|iPod/i.test(navigator.userAgent);
const canShare = navigator.share && navigator.canShare;

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    setupTabs();
    setupUpload();
    setupViewer();
    setupSelection();
    loadPhotos();
});

// ==================== TABS ====================

function setupTabs() {
    document.querySelectorAll('.tab').forEach(tab => {
        tab.addEventListener('click', () => {
            document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
            tab.classList.add('active');
            currentTab = tab.dataset.tab;
            exitSelectMode();
            loadPhotos();
        });
    });
}

// ==================== PHOTOS ====================

async function loadPhotos() {
    const gallery = document.getElementById('gallery');
    gallery.innerHTML = '<div class="loading">Loading photos...</div>';

    const endpoints = {
        'my-photos': '/api/photos/my',
        'family': '/api/photos/shared',
        'all': '/api/photos/all'
    };

    try {
        const response = await fetch(endpoints[currentTab] || endpoints['my-photos']);
        
        if (response.status === 401) {
            window.location.href = '/login';
            return;
        }
        
        if (!response.ok) throw new Error('Failed to load photos');
        
        currentPhotos = await response.json() || [];
        renderGallery();
    } catch (error) {
        console.error('Error:', error);
        gallery.innerHTML = '<div class="loading" style="color: var(--danger);">Failed to load photos</div>';
    }
}

function renderGallery() {
    const gallery = document.getElementById('gallery');
    const emptyState = document.getElementById('emptyState');
    const emptyTitle = document.getElementById('emptyTitle');
    const emptyMessage = document.getElementById('emptyMessage');

    if (!currentPhotos.length) {
        gallery.style.display = 'none';
        emptyState.style.display = 'block';
        
        const messages = {
            'my-photos': ['No Photos Yet', 'Upload your first photo to get started'],
            'family': ['Family Area Empty', 'No shared photos yet'],
            'all': ['No Photos', 'No photos in the system']
        };
        
        const [title, msg] = messages[currentTab] || messages['my-photos'];
        emptyTitle.textContent = title;
        emptyMessage.textContent = msg;
        return;
    }

    gallery.style.display = 'grid';
    emptyState.style.display = 'none';

    currentPhotos.sort((a, b) => new Date(b.uploaded_at) - new Date(a.uploaded_at));

    gallery.innerHTML = currentPhotos.map((photo, i) => `
        <div class="photo-card ${selectedPhotos.has(photo.id) ? 'selected' : ''}" 
             data-photo-id="${photo.id}"
             onclick="${selectMode ? `togglePhotoSelection(${photo.id})` : `openViewer(${i})`}">
            ${selectMode ? `
                <div class="photo-checkbox ${selectedPhotos.has(photo.id) ? 'checked' : ''}" onclick="event.stopPropagation(); togglePhotoSelection(${photo.id})">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
                        <polyline points="20 6 9 17 4 12"/>
                    </svg>
                </div>
            ` : ''}
            <img src="${esc(photo.thumbnail_url)}" alt="${esc(photo.filename)}" loading="lazy">
            <div class="photo-card-overlay">
                <div class="photo-card-name">${esc(photo.filename)}</div>
                <div class="photo-card-meta">
                    ${formatSize(photo.size)}
                    ${photo.username && currentTab !== 'my-photos' ? `<span class="badge">${esc(photo.username)}</span>` : ''}
                    ${photo.is_shared ? '<span class="badge badge-success">Shared</span>' : ''}
                </div>
            </div>
        </div>
    `).join('');
}

// ==================== UPLOAD ====================

function setupUpload() {
    const uploadBtn = document.getElementById('uploadBtn');
    const uploadArea = document.getElementById('uploadArea');
    const uploadBox = document.getElementById('uploadBox');
    const closeUpload = document.getElementById('closeUpload');
    const fileInput = document.getElementById('fileInput');

    uploadBtn?.addEventListener('click', () => uploadArea.style.display = 'flex');
    closeUpload?.addEventListener('click', () => uploadArea.style.display = 'none');
    
    uploadArea?.addEventListener('click', (e) => {
        if (e.target === uploadArea) uploadArea.style.display = 'none';
    });

    uploadBox?.addEventListener('click', (e) => {
        if (e.target !== closeUpload && !closeUpload?.contains(e.target)) {
            fileInput?.click();
        }
    });

    fileInput?.addEventListener('change', (e) => handleUpload(e.target.files));

    uploadBox?.addEventListener('dragover', (e) => {
        e.preventDefault();
        uploadBox.classList.add('drag-over');
    });

    uploadBox?.addEventListener('dragleave', () => uploadBox.classList.remove('drag-over'));

    uploadBox?.addEventListener('drop', (e) => {
        e.preventDefault();
        uploadBox.classList.remove('drag-over');
        handleUpload(e.dataTransfer.files);
    });
}

async function handleUpload(files) {
    if (!files?.length) return;

    const uploadBox = document.getElementById('uploadBox');
    const uploadProgress = document.getElementById('uploadProgress');
    const progressFill = document.getElementById('progressFill');
    const progressText = document.getElementById('progressText');

    uploadBox.style.display = 'none';
    uploadProgress.style.display = 'block';

    let completed = 0;
    const total = Array.from(files).filter(f => f.type.startsWith('image/')).length;

    for (const file of files) {
        if (!file.type.startsWith('image/')) continue;

        try {
            const formData = new FormData();
            formData.append('photo', file);

            const response = await fetch('/api/photos/upload', {
                method: 'POST',
                headers: { 'X-CSRF-Token': csrfToken },
                body: formData
            });

            if (!response.ok) throw new Error('Upload failed');
            
            completed++;
            progressFill.style.width = `${(completed / total) * 100}%`;
            progressText.textContent = `Uploading ${completed}/${total}...`;
        } catch (error) {
            console.error('Upload error:', file.name, error);
        }
    }

    document.getElementById('uploadArea').style.display = 'none';
    uploadBox.style.display = 'block';
    uploadProgress.style.display = 'none';
    progressFill.style.width = '0%';
    document.getElementById('fileInput').value = '';

    document.querySelector('[data-tab="my-photos"]')?.click();
}

// ==================== VIEWER ====================

function setupViewer() {
    const viewer = document.getElementById('viewer');
    const viewerImage = document.getElementById('viewerImage');

    document.getElementById('viewerClose')?.addEventListener('click', closeViewer);
    document.getElementById('viewerPrev')?.addEventListener('click', () => navigate(-1));
    document.getElementById('viewerNext')?.addEventListener('click', () => navigate(1));
    document.getElementById('viewerZoomIn')?.addEventListener('click', () => zoom(0.25));
    document.getElementById('viewerZoomOut')?.addEventListener('click', () => zoom(-0.25));
    document.getElementById('viewerZoomReset')?.addEventListener('click', resetZoom);
    document.getElementById('viewerShare')?.addEventListener('click', toggleShare);
    document.getElementById('viewerDelete')?.addEventListener('click', deletePhoto);
    document.getElementById('viewerSave')?.addEventListener('click', saveToPhotos);

    // Keyboard
    document.addEventListener('keydown', (e) => {
        if (viewer?.style.display === 'none') return;
        switch (e.key) {
            case 'Escape': closeViewer(); break;
            case 'ArrowLeft': navigate(-1); break;
            case 'ArrowRight': navigate(1); break;
            case '+': case '=': zoom(0.25); break;
            case '-': zoom(-0.25); break;
            case '0': resetZoom(); break;
        }
    });

    // Mouse wheel zoom
    viewerImage?.addEventListener('wheel', (e) => {
        if (viewer?.style.display === 'none') return;
        e.preventDefault();
        zoom(e.deltaY > 0 ? -0.1 : 0.1);
    });

    // Drag to pan
    viewerImage?.addEventListener('mousedown', (e) => {
        if (zoomLevel <= 1) return;
        isDragging = true;
        dragStart = { x: e.clientX - imageOffset.x, y: e.clientY - imageOffset.y };
        viewerImage.style.cursor = 'grabbing';
    });

    document.addEventListener('mousemove', (e) => {
        if (!isDragging) return;
        imageOffset = { x: e.clientX - dragStart.x, y: e.clientY - dragStart.y };
        applyTransform();
    });

    document.addEventListener('mouseup', () => {
        isDragging = false;
        if (viewerImage && zoomLevel > 1) viewerImage.style.cursor = 'grab';
    });

    // Touch swipe
    let touchStart = { x: 0, y: 0 };
    viewerImage?.addEventListener('touchstart', (e) => {
        touchStart = { x: e.touches[0].clientX, y: e.touches[0].clientY };
    });

    viewerImage?.addEventListener('touchend', (e) => {
        if (zoomLevel > 1) return;
        const diff = e.changedTouches[0].clientX - touchStart.x;
        if (Math.abs(diff) > 50) navigate(diff > 0 ? -1 : 1);
    });

    // Image load
    viewerImage?.addEventListener('load', () => {
        document.getElementById('viewerLoading').style.display = 'none';
        viewerImage.style.opacity = '1';
    });
}

function openViewer(index) {
    if (index < 0 || index >= currentPhotos.length) return;

    currentPhotoIndex = index;
    const photo = currentPhotos[index];
    const viewer = document.getElementById('viewer');
    const viewerImage = document.getElementById('viewerImage');

    resetZoom();

    document.getElementById('viewerLoading').style.display = 'flex';
    viewerImage.style.opacity = '0';
    viewerImage.src = photo.original_url;

    document.getElementById('viewerFilename').textContent = photo.filename;
    
    const meta = [];
    if (photo.username && photo.user_id !== currentUserID) meta.push(`by ${photo.username}`);
    if (photo.is_shared) meta.push('Shared');
    document.getElementById('viewerMeta').textContent = meta.join(' • ');

    document.getElementById('viewerCounter').textContent = `${index + 1} / ${currentPhotos.length}`;

    const downloadBtn = document.getElementById('viewerDownload');
    downloadBtn.href = photo.original_url;
    downloadBtn.download = photo.filename;

    // On iOS, show save button instead of download (goes to Photos app via share sheet)
    const saveBtn = document.getElementById('viewerSave');
    if (saveBtn) {
        if (isIOS && canShare) {
            saveBtn.style.display = 'flex';
            downloadBtn.style.display = 'none';
        } else {
            saveBtn.style.display = 'none';
            downloadBtn.style.display = 'flex';
        }
    }

    const shareBtn = document.getElementById('viewerShare');
    if (photo.user_id === currentUserID) {
        shareBtn.style.display = 'flex';
        shareBtn.querySelector('span:last-child').textContent = photo.is_shared ? 'Unshare' : 'Share';
    } else {
        shareBtn.style.display = 'none';
    }

    const deleteBtn = document.getElementById('viewerDelete');
    deleteBtn.style.display = (photo.user_id === currentUserID || isAdmin) ? 'flex' : 'none';

    document.getElementById('viewerPrev').style.visibility = index > 0 ? 'visible' : 'hidden';
    document.getElementById('viewerNext').style.visibility = index < currentPhotos.length - 1 ? 'visible' : 'hidden';

    viewer.style.display = 'flex';
    document.body.style.overflow = 'hidden';
}

function closeViewer() {
    document.getElementById('viewer').style.display = 'none';
    document.body.style.overflow = '';
    currentPhotoIndex = -1;
    resetZoom();
}

function navigate(dir) {
    const newIndex = currentPhotoIndex + dir;
    if (newIndex >= 0 && newIndex < currentPhotos.length) openViewer(newIndex);
}

function zoom(delta) {
    zoomLevel = Math.max(0.5, Math.min(5, zoomLevel + delta));
    applyTransform();
}

function resetZoom() {
    zoomLevel = 1;
    imageOffset = { x: 0, y: 0 };
    applyTransform();
}

function applyTransform() {
    const img = document.getElementById('viewerImage');
    if (img) {
        img.style.transform = `scale(${zoomLevel}) translate(${imageOffset.x}px, ${imageOffset.y}px)`;
        img.style.cursor = zoomLevel > 1 ? 'grab' : 'default';
    }
}

async function toggleShare() {
    if (currentPhotoIndex < 0) return;
    const photo = currentPhotos[currentPhotoIndex];

    try {
        const response = await fetch(`/api/photos/${photo.id}/share`, {
            method: 'POST',
            headers: { 'X-CSRF-Token': csrfToken }
        });

        if (!response.ok) throw new Error('Failed');

        const result = await response.json();
        photo.is_shared = result.is_shared;

        document.getElementById('viewerShare').querySelector('span:last-child').textContent = 
            result.is_shared ? 'Unshare' : 'Share';
        document.getElementById('viewerMeta').textContent = result.is_shared ? 'Shared' : '';

        renderGallery();
    } catch (error) {
        alert('Failed to update sharing');
    }
}

async function deletePhoto() {
    if (currentPhotoIndex < 0) return;
    const photo = currentPhotos[currentPhotoIndex];

    if (!confirm(`Delete "${photo.filename}"?`)) return;

    try {
        const response = await fetch(`/api/photos/${photo.id}`, {
            method: 'DELETE',
            headers: { 'X-CSRF-Token': csrfToken }
        });

        if (!response.ok) throw new Error('Failed');

        currentPhotos.splice(currentPhotoIndex, 1);

        if (!currentPhotos.length) {
            closeViewer();
        } else {
            openViewer(Math.min(currentPhotoIndex, currentPhotos.length - 1));
        }

        renderGallery();
    } catch (error) {
        alert('Failed to delete photo');
    }
}

// Save to Photos (iOS) - Uses Web Share API to open share sheet
async function saveToPhotos() {
    if (currentPhotoIndex < 0) return;
    const photo = currentPhotos[currentPhotoIndex];

    try {
        // Fetch the image as a blob
        const response = await fetch(photo.original_url);
        const blob = await response.blob();

        // Create a file from the blob
        const file = new File([blob], photo.filename, { type: blob.type });

        // Check if we can share files
        if (navigator.canShare && navigator.canShare({ files: [file] })) {
            await navigator.share({
                files: [file],
                title: photo.filename
            });
        } else {
            // Fallback: open image in new tab for long-press saving
            window.open(photo.original_url, '_blank');
            alert('Long-press the image and select "Add to Photos" to save');
        }
    } catch (error) {
        if (error.name !== 'AbortError') {
            console.error('Save error:', error);
            // Fallback
            window.open(photo.original_url, '_blank');
            alert('Long-press the image and select "Add to Photos" to save');
        }
    }
}

// ==================== UTILITIES ====================

function esc(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

function formatSize(bytes) {
    if (!bytes) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
}

// ==================== SELECTION ====================

function setupSelection() {
    document.getElementById('selectBtn')?.addEventListener('click', toggleSelectMode);
    document.getElementById('cancelSelectBtn')?.addEventListener('click', exitSelectMode);
    document.getElementById('selectAllBtn')?.addEventListener('click', selectAll);
    document.getElementById('deselectAllBtn')?.addEventListener('click', deselectAll);
    document.getElementById('bulkDownloadBtn')?.addEventListener('click', bulkDownload);
    document.getElementById('bulkShareBtn')?.addEventListener('click', () => bulkShare(true));
    document.getElementById('bulkUnshareBtn')?.addEventListener('click', () => bulkShare(false));
    document.getElementById('bulkDeleteBtn')?.addEventListener('click', bulkDelete);
}

function toggleSelectMode() {
    selectMode = !selectMode;
    const selectBtn = document.getElementById('selectBtn');
    const selectionBar = document.getElementById('selectionBar');
    
    if (selectMode) {
        selectBtn.classList.add('active');
        selectBtn.innerHTML = `
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
            </svg>
            Cancel
        `;
        selectionBar.style.display = 'flex';
    } else {
        exitSelectMode();
    }
    renderGallery();
}

function exitSelectMode() {
    selectMode = false;
    selectedPhotos.clear();
    
    const selectBtn = document.getElementById('selectBtn');
    const selectionBar = document.getElementById('selectionBar');
    
    if (selectBtn) {
        selectBtn.classList.remove('active');
        selectBtn.innerHTML = `
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <polyline points="9 11 12 14 22 4"/>
                <path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"/>
            </svg>
            Select
        `;
    }
    if (selectionBar) {
        selectionBar.style.display = 'none';
    }
    
    updateSelectionCount();
    renderGallery();
}

function togglePhotoSelection(photoId) {
    if (selectedPhotos.has(photoId)) {
        selectedPhotos.delete(photoId);
    } else {
        selectedPhotos.add(photoId);
    }
    updateSelectionCount();
    renderGallery();
}

function selectAll() {
    currentPhotos.forEach(p => selectedPhotos.add(p.id));
    updateSelectionCount();
    renderGallery();
}

function deselectAll() {
    selectedPhotos.clear();
    updateSelectionCount();
    renderGallery();
}

function updateSelectionCount() {
    const countEl = document.getElementById('selectionCount');
    if (countEl) {
        const count = selectedPhotos.size;
        countEl.textContent = `${count} selected`;
    }
    
    // Enable/disable bulk action buttons based on selection
    const hasSelection = selectedPhotos.size > 0;
    document.getElementById('bulkDownloadBtn')?.classList.toggle('disabled', !hasSelection);
    document.getElementById('bulkShareBtn')?.classList.toggle('disabled', !hasSelection);
    document.getElementById('bulkUnshareBtn')?.classList.toggle('disabled', !hasSelection);
    document.getElementById('bulkDeleteBtn')?.classList.toggle('disabled', !hasSelection);
}

async function bulkDownload() {
    if (selectedPhotos.size === 0) {
        alert('Please select photos to download');
        return;
    }

    try {
        const response = await fetch('/api/photos/bulk/download', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'X-CSRF-Token': csrfToken
            },
            body: JSON.stringify({ photo_ids: Array.from(selectedPhotos) })
        });

        if (!response.ok) throw new Error('Download failed');

        // Get filename from Content-Disposition header or use default
        const disposition = response.headers.get('Content-Disposition');
        let filename = 'mnemosyne_photos.zip';
        if (disposition) {
            const match = disposition.match(/filename="?([^"]+)"?/);
            if (match) filename = match[1];
        }

        // Create blob and download
        const blob = await response.blob();
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        window.URL.revokeObjectURL(url);
        a.remove();

        exitSelectMode();
    } catch (error) {
        console.error('Bulk download error:', error);
        alert('Failed to download photos');
    }
}

async function bulkShare(share) {
    if (selectedPhotos.size === 0) {
        alert('Please select photos to ' + (share ? 'share' : 'unshare'));
        return;
    }

    // Only allow sharing own photos
    const ownPhotos = currentPhotos.filter(p => 
        selectedPhotos.has(p.id) && p.user_id === currentUserID
    );

    if (ownPhotos.length === 0) {
        alert('You can only share/unshare your own photos');
        return;
    }

    try {
        const response = await fetch('/api/photos/bulk/share', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'X-CSRF-Token': csrfToken
            },
            body: JSON.stringify({
                photo_ids: ownPhotos.map(p => p.id),
                share: share
            })
        });

        if (!response.ok) throw new Error('Share operation failed');

        const result = await response.json();
        alert(result.message);

        exitSelectMode();
        loadPhotos();
    } catch (error) {
        console.error('Bulk share error:', error);
        alert('Failed to ' + (share ? 'share' : 'unshare') + ' photos');
    }
}

async function bulkDelete() {
    if (selectedPhotos.size === 0) {
        alert('Please select photos to delete');
        return;
    }

    // Filter to only photos user can delete (own or admin)
    const deletablePhotos = currentPhotos.filter(p => 
        selectedPhotos.has(p.id) && (p.user_id === currentUserID || isAdmin)
    );

    if (deletablePhotos.length === 0) {
        alert('You can only delete your own photos');
        return;
    }

    const count = deletablePhotos.length;
    if (!confirm(`Are you sure you want to delete ${count} photo${count > 1 ? 's' : ''}? This cannot be undone.`)) {
        return;
    }

    try {
        const response = await fetch('/api/photos/bulk/delete', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'X-CSRF-Token': csrfToken
            },
            body: JSON.stringify({
                photo_ids: deletablePhotos.map(p => p.id)
            })
        });

        if (!response.ok) throw new Error('Delete operation failed');

        const result = await response.json();
        alert(result.message);

        exitSelectMode();
        loadPhotos();
    } catch (error) {
        console.error('Bulk delete error:', error);
        alert('Failed to delete photos');
    }
}
