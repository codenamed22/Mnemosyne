// Admin Panel JavaScript

const csrfToken = document.getElementById('csrfToken')?.value || '';
let confirmCallback = null;

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    loadStats();
    loadUsers();
    setupConfirmModal();
});

// Load system stats
async function loadStats() {
    try {
        const response = await fetch('/api/admin/stats');
        if (!response.ok) throw new Error('Failed to load stats');

        const stats = await response.json();
        document.getElementById('totalUsers').textContent = stats.total_users;
        document.getElementById('totalPhotos').textContent = stats.total_photos;
    } catch (error) {
        console.error('Error loading stats:', error);
    }
}

// Load users list
async function loadUsers() {
    const usersList = document.getElementById('usersList');

    try {
        const response = await fetch('/api/admin/users');
        if (!response.ok) throw new Error('Failed to load users');

        const users = await response.json();
        displayUsers(users);
    } catch (error) {
        console.error('Error loading users:', error);
        usersList.innerHTML = '<div class="error">Failed to load users</div>';
    }
}

// Display users
function displayUsers(users) {
    const usersList = document.getElementById('usersList');

    if (!users || users.length === 0) {
        usersList.innerHTML = '<p>No users found.</p>';
        return;
    }

    usersList.innerHTML = `
        <table class="users-table">
            <thead>
                <tr>
                    <th>Username</th>
                    <th>Role</th>
                    <th>Photos</th>
                    <th>Joined</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody>
                ${users.map(user => `
                    <tr data-user-id="${user.id}">
                        <td>${escapeHtml(user.username)}</td>
                        <td>
                            <span class="role-badge ${user.role}">${user.role}</span>
                        </td>
                        <td>${user.photo_count}</td>
                        <td>${formatDate(user.created_at)}</td>
                        <td class="actions">
                            <button class="btn-small btn-secondary" onclick="toggleRole(${user.id}, '${user.role}')">
                                ${user.role === 'admin' ? 'Make User' : 'Make Admin'}
                            </button>
                            <button class="btn-small btn-danger" onclick="confirmDeleteUser(${user.id}, '${escapeHtml(user.username)}')">
                                Delete
                            </button>
                        </td>
                    </tr>
                `).join('')}
            </tbody>
        </table>
    `;
}

// Toggle user role
async function toggleRole(userId, currentRole) {
    const newRole = currentRole === 'admin' ? 'user' : 'admin';

    try {
        const response = await fetch(`/api/admin/users/${userId}/role`, {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
                'X-CSRF-Token': csrfToken
            },
            body: JSON.stringify({ role: newRole })
        });

        if (!response.ok) {
            const error = await response.text();
            throw new Error(error);
        }

        loadUsers();
    } catch (error) {
        console.error('Error updating role:', error);
        alert('Failed to update role: ' + error.message);
    }
}

// Confirm delete user
function confirmDeleteUser(userId, username) {
    showConfirm(
        'Delete User',
        `Are you sure you want to delete user "${username}"? All their photos will also be deleted.`,
        () => deleteUser(userId)
    );
}

// Delete user
async function deleteUser(userId) {
    try {
        const response = await fetch(`/api/admin/users/${userId}`, {
            method: 'DELETE',
            headers: {
                'X-CSRF-Token': csrfToken
            }
        });

        if (!response.ok) {
            const error = await response.text();
            throw new Error(error);
        }

        loadUsers();
        loadStats();
    } catch (error) {
        console.error('Error deleting user:', error);
        alert('Failed to delete user: ' + error.message);
    }
}

// Setup confirm modal
function setupConfirmModal() {
    const modal = document.getElementById('confirmModal');
    const overlay = modal.querySelector('.modal-overlay');
    const cancelBtn = document.getElementById('confirmCancel');
    const okBtn = document.getElementById('confirmOk');

    overlay.addEventListener('click', hideConfirm);
    cancelBtn.addEventListener('click', hideConfirm);
    okBtn.addEventListener('click', () => {
        if (confirmCallback) {
            confirmCallback();
        }
        hideConfirm();
    });
}

// Show confirm modal
function showConfirm(title, message, callback) {
    const modal = document.getElementById('confirmModal');
    document.getElementById('confirmTitle').textContent = title;
    document.getElementById('confirmMessage').textContent = message;
    confirmCallback = callback;
    modal.style.display = 'block';
}

// Hide confirm modal
function hideConfirm() {
    const modal = document.getElementById('confirmModal');
    modal.style.display = 'none';
    confirmCallback = null;
}

// Utility functions
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function formatDate(dateString) {
    const date = new Date(dateString);
    return date.toLocaleDateString();
}

